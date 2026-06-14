package service

import (
	"BUPT_EC/cache"
	"BUPT_EC/config"
	"BUPT_EC/logs"
	"BUPT_EC/service/model"
	"BUPT_EC/utils"
	"context"
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	ServerConfigURL = "https://jwglweixin.bupt.edu.cn/sjd/serverconfig.json"
	DefaultAPIURL   = "https://jwglweixin.bupt.edu.cn/bjyddx/"

	LoginUsernameKey = "JW_USERNAME"
	LoginPasswordKey = "JW_PASSWORD"
	LoginTokenKey    = "JW_TOKEN"

	TodayCacheKey = "TODAY_CLASSROOMS_CACHE"

	tokenPasswordKey = "qzkj1kjghd=876&*"

	jwRequestTimeout      = 12 * time.Second
	classroomFreshTTL     = 5 * time.Minute
	classroomRefreshLimit = 30 * time.Second
)

var (
	ErrNoTodayCache = errors.New("no today classroom cache")

	tokenManager = &TokenManager{}
	refreshGroup = singleflight.Group{}

	roomPattern = regexp.MustCompile(`^(.+)[(（](\d+)[)）]$`)
)

type TokenManager struct {
	mu          sync.Mutex
	token       string
	apiURL      string
	tokenGroup  singleflight.Group
	apiURLGroup singleflight.Group
}

type jwErrorKind string

const (
	jwErrorAuth     jwErrorKind = "jw_auth_failed"
	jwErrorConfig   jwErrorKind = "jw_config_error"
	jwErrorLogin    jwErrorKind = "jw_login_failed"
	jwErrorQuery    jwErrorKind = "jw_query_failed"
	jwErrorParse    jwErrorKind = "jw_bad_response"
	jwErrorUpstream jwErrorKind = "jw_unavailable"
)

type jwError struct {
	kind jwErrorKind
	op   string
	err  error
	msg  string
}

func (e *jwError) Error() string {
	if e == nil {
		return ""
	}
	if e.err != nil && e.msg != "" {
		return fmt.Sprintf("%s: %s: %v", e.op, e.msg, e.err)
	}
	if e.err != nil {
		return fmt.Sprintf("%s: %v", e.op, e.err)
	}
	if e.msg != "" {
		return fmt.Sprintf("%s: %s", e.op, e.msg)
	}
	return e.op
}

func (e *jwError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func newJWError(kind jwErrorKind, op string, err error, format string, v ...interface{}) error {
	return &jwError{kind: kind, op: op, err: err, msg: fmt.Sprintf(format, v...)}
}

type RuntimeStatus struct {
	LastLoginSuccessAt   *time.Time `json:"last_login_success_at,omitempty"`
	LastLoginError       string     `json:"last_login_error,omitempty"`
	LastRefreshSuccessAt *time.Time `json:"last_refresh_success_at,omitempty"`
	LastRefreshError     string     `json:"last_refresh_error,omitempty"`
	CacheAvailable       bool       `json:"cache_available"`
	CacheFresh           bool       `json:"cache_fresh"`
	CacheStale           bool       `json:"cache_stale"`
	CacheDate            string     `json:"cache_date,omitempty"`
}

var (
	runtimeStatusMu sync.RWMutex
	runtimeStatus   RuntimeStatus
)

func ResetRuntimeStateForTest() {
	tokenManager = &TokenManager{}
	refreshGroup = singleflight.Group{}
	runtimeStatusMu.Lock()
	runtimeStatus = RuntimeStatus{}
	runtimeStatusMu.Unlock()
	cache.DeleteCache(TodayCacheKey)
}

func Login(ctx context.Context) error {
	_, err := tokenManager.EnsureToken(ctx, true)
	return err
}

func QueryOne(ctx context.Context, id string) ([]model.JWClassInfo, error) {
	return queryCampus(ctx, id, false)
}

func QueryAll(ctx context.Context) (*model.TodayClassrooms, error) {
	return refreshTodayClassrooms(ctx)
}

func GetTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error) {
	if cached, ok := getCachedTodayClassrooms(); ok && !cached.ExpiresAt.Before(time.Now()) {
		resp := cloneTodayClassrooms(cached)
		resp.Stale = false
		resp.Error = nil
		return resp, nil
	}

	value, err, _ := refreshGroup.Do(TodayCacheKey, func() (interface{}, error) {
		refreshCtx, cancel := context.WithTimeout(ctx, classroomRefreshLimit)
		defer cancel()
		return refreshTodayClassrooms(refreshCtx)
	})
	if err == nil {
		fresh, ok := value.(*model.TodayClassrooms)
		if !ok || fresh == nil {
			return nil, newJWError(jwErrorParse, "classroom refresh", nil, "unexpected refresh result")
		}
		return cloneTodayClassrooms(fresh), nil
	}

	if cached, ok := getCachedTodayClassrooms(); ok && time.Now().Before(cached.StaleUntil) {
		resp := cloneTodayClassrooms(cached)
		resp.Stale = true
		resp.Error = &model.APIError{
			Type:    classifyError(err),
			Message: "教务系统暂时不可用，当前展示的是今天最后一次成功刷新数据",
		}
		return resp, nil
	}

	return nil, err
}

func refreshTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error) {
	startedAt := time.Now()
	today, err := doRefreshTodayClassrooms(ctx)
	if err != nil {
		recordRefreshFailure(err)
		logs.CtxWarn(ctx, "classroom refresh failed after %s: %v", time.Since(startedAt), err)
		return nil, err
	}
	recordRefreshSuccess(time.Now())
	logs.CtxInfo(ctx, "classroom refresh succeeded in %s", time.Since(startedAt))
	return today, nil
}

func doRefreshTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error) {
	now := time.Now()
	today := &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now,
		ExpiresAt:  now.Add(classroomFreshTTL),
		StaleUntil: endOfDay(now),
		Stale:      false,
		Campuses:   []model.CampusInfo{},
		Error:      nil,
	}

	for _, campusConfig := range config.GetConfig().Campuses {
		jwRows, err := queryCampus(ctx, campusConfig.ID, false)
		if err != nil {
			return nil, err
		}
		campus := buildCampusInfo(campusConfig, jwRows)
		today.Campuses = append(today.Campuses, campus)
	}

	cache.SetCache(TodayCacheKey, today, time.Until(today.StaleUntil))
	return cloneTodayClassrooms(today), nil
}

func queryCampus(ctx context.Context, campusID string, forceRefresh bool) ([]model.JWClassInfo, error) {
	token, err := tokenManager.EnsureToken(ctx, forceRefresh)
	if err != nil {
		return nil, err
	}

	rows, err := queryCampusWithToken(ctx, campusID, token)
	if err == nil {
		return rows, nil
	}
	if !isJWErrorKind(err, jwErrorAuth) {
		return nil, err
	}

	token, refreshErr := tokenManager.EnsureToken(ctx, true)
	if refreshErr != nil {
		return nil, errors.Join(err, refreshErr)
	}
	rows, retryErr := queryCampusWithToken(ctx, campusID, token)
	if retryErr != nil {
		return nil, errors.Join(err, retryErr)
	}
	return rows, nil
}

func queryCampusWithToken(ctx context.Context, campusID string, token string) ([]model.JWClassInfo, error) {
	requestCtx, cancel := context.WithTimeout(ctx, jwRequestTimeout)
	defer cancel()

	apiURL, err := tokenManager.APIURL(requestCtx)
	if err != nil {
		return nil, err
	}
	queryURL, err := joinAPIPath(apiURL, "todayClassrooms")
	if err != nil {
		return nil, newJWError(jwErrorConfig, "jw query", err, "build query URL failed")
	}
	queryURL = addQuery(queryURL, map[string]string{"campusId": campusID})

	code, _, body, err := utils.HttpPostWithHeader(requestCtx, queryURL, map[string]string{"token": token})
	if err != nil {
		return nil, newJWError(jwErrorQuery, "jw query", err, "request failed")
	}
	if code != 200 {
		kind := jwErrorQuery
		if code == http.StatusUnauthorized || code == http.StatusForbidden {
			kind = jwErrorAuth
		}
		return nil, newJWError(kind, "jw query", nil, "http status %d", code)
	}

	var resp model.QueryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, newJWError(jwErrorParse, "jw query", err, "response invalid")
	}
	if resp.Code != "1" {
		kind := jwErrorQuery
		if isAuthFailureMessage(resp.Msg) {
			kind = jwErrorAuth
		}
		return nil, newJWError(kind, "jw query", nil, "%s", safeRemoteMessage(resp.Msg))
	}
	return resp.Data, nil
}

func (m *TokenManager) EnsureToken(ctx context.Context, forceRefresh bool) (string, error) {
	if !forceRefresh {
		if token := os.Getenv(LoginTokenKey); token != "" {
			m.setToken(token)
			return token, nil
		}
		if token := m.cachedToken(); token != "" {
			return token, nil
		}
	}

	flightKey := "jw-token"
	if forceRefresh {
		flightKey = "jw-token-force"
	}

	value, err, _ := m.tokenGroup.Do(flightKey, func() (interface{}, error) {
		if !forceRefresh {
			if token := os.Getenv(LoginTokenKey); token != "" {
				m.setToken(token)
				return token, nil
			}
			if token := m.cachedToken(); token != "" {
				return token, nil
			}
		}

		startedAt := time.Now()
		token, err := m.refreshToken(ctx)
		if err != nil {
			recordLoginFailure(err)
			logs.CtxWarn(ctx, "jw login failed after %s: %v", time.Since(startedAt), err)
			return "", err
		}
		m.setToken(token)
		recordLoginSuccess(time.Now())
		logs.CtxInfo(ctx, "jw login succeeded in %s", time.Since(startedAt))
		return token, nil
	})
	if err != nil {
		return "", err
	}
	token, ok := value.(string)
	if !ok || token == "" {
		return "", newJWError(jwErrorLogin, "jw login", nil, "unexpected token result")
	}
	return token, nil
}

func (m *TokenManager) APIURL(ctx context.Context) (string, error) {
	if apiURL := m.cachedAPIURL(); apiURL != "" {
		return apiURL, nil
	}

	value, err, _ := m.apiURLGroup.Do("jw-api-url", func() (interface{}, error) {
		if apiURL := m.cachedAPIURL(); apiURL != "" {
			return apiURL, nil
		}
		apiURL, err := m.fetchAPIURL(ctx)
		if err != nil {
			return "", err
		}
		m.setAPIURL(apiURL)
		return apiURL, nil
	})
	if err != nil {
		return "", err
	}
	apiURL, ok := value.(string)
	if !ok || apiURL == "" {
		return "", newJWError(jwErrorConfig, "serverconfig", nil, "unexpected API URL result")
	}
	return apiURL, nil
}

func (m *TokenManager) cachedToken() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.token
}

func (m *TokenManager) setToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = token
}

func (m *TokenManager) cachedAPIURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.apiURL
}

func (m *TokenManager) setAPIURL(apiURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.apiURL = apiURL
}

func (m *TokenManager) refreshToken(ctx context.Context) (string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, jwRequestTimeout)
	defer cancel()

	apiURL, err := m.APIURL(requestCtx)
	if err != nil {
		return "", err
	}
	loginURL, err := joinAPIPath(apiURL, "login")
	if err != nil {
		return "", newJWError(jwErrorConfig, "jw login", err, "build login URL failed")
	}

	userNo := os.Getenv(LoginUsernameKey)
	password := os.Getenv(LoginPasswordKey)
	if userNo == "" || password == "" {
		return "", newJWError(jwErrorConfig, "jw login", nil, "JW_USERNAME or JW_PASSWORD is not configured")
	}

	encryptedPassword, err := encryptJWPassword(password)
	if err != nil {
		return "", newJWError(jwErrorConfig, "jw login", err, "encrypt login password failed")
	}
	req := map[string]string{
		"userNo":      userNo,
		"pwd":         encryptedPassword,
		"encode":      "1",
		"captchaData": "",
		"codeVal":     "",
	}

	code, _, body, err := utils.HttpPostForm(requestCtx, loginURL, req)
	if err != nil {
		return "", newJWError(jwErrorLogin, "jw login", err, "request failed")
	}
	if code != http.StatusOK {
		return "", newJWError(jwErrorLogin, "jw login", nil, "http status %d", code)
	}

	var resp model.LoginResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", newJWError(jwErrorParse, "jw login", err, "response invalid")
	}
	if resp.Code != "1" || resp.Data.Token == "" {
		return "", newJWError(jwErrorLogin, "jw login", nil, "%s", safeRemoteMessage(resp.Msg))
	}
	return resp.Data.Token, nil
}

func (m *TokenManager) fetchAPIURL(ctx context.Context) (string, error) {
	requestCtx, cancel := context.WithTimeout(ctx, jwRequestTimeout)
	defer cancel()

	code, _, body, err := utils.HttpGet(requestCtx, ServerConfigURL)
	if err != nil {
		logs.CtxWarn(ctx, "serverconfig request failed; using default HTTPS API URL: %v", err)
		return validateJWAPIURL(DefaultAPIURL)
	}
	if code != http.StatusOK {
		logs.CtxWarn(ctx, "serverconfig http status %d; using default HTTPS API URL", code)
		return validateJWAPIURL(DefaultAPIURL)
	}

	var resp model.ServerConfigResponse
	if err := json.Unmarshal(body, &resp); err != nil || resp.APIURL == "" {
		logs.CtxWarn(ctx, "serverconfig invalid; using default HTTPS API URL")
		return validateJWAPIURL(DefaultAPIURL)
	}
	apiURL, err := validateJWAPIURL(resp.APIURL)
	if err != nil {
		logs.CtxWarn(ctx, "serverconfig API URL rejected; using default HTTPS API URL: %v", err)
		return validateJWAPIURL(DefaultAPIURL)
	}
	return apiURL, nil
}

func buildCampusInfo(campusConfig config.CampusConfig, rows []model.JWClassInfo) model.CampusInfo {
	buildingMap := map[string]map[string]*model.RoomInfo{}
	nodeRooms := map[int]map[string]struct{}{}
	nodeTimes := map[int]string{}

	for _, row := range rows {
		node, err := strconv.Atoi(row.NodeName)
		if err != nil {
			continue
		}
		nodeTimes[node] = row.NodeTime
		if _, exists := nodeRooms[node]; !exists {
			nodeRooms[node] = map[string]struct{}{}
		}
		classrooms := strings.Split(row.Classrooms, ",")
		for _, raw := range classrooms {
			buildingName, roomName, displayName, capacity, ok := parseRoom(raw)
			if !ok {
				continue
			}
			nodeRooms[node][displayName] = struct{}{}
			if _, exists := buildingMap[buildingName]; !exists {
				buildingMap[buildingName] = map[string]*model.RoomInfo{}
			}
			room := buildingMap[buildingName][displayName]
			if room == nil {
				room = &model.RoomInfo{
					Name:        roomName,
					DisplayName: displayName,
					Capacity:    capacity,
					FreeNodes:   []int{},
					FreeTimes:   []model.FreeTime{},
				}
				buildingMap[buildingName][displayName] = room
			}
			room.FreeNodes = append(room.FreeNodes, node)
			room.FreeTimes = append(room.FreeTimes, model.FreeTime{Node: node, Time: row.NodeTime})
		}
	}

	nodes := make([]model.NodeInfo, 0, len(nodeTimes))
	for node, nodeTime := range nodeTimes {
		nodes = append(nodes, model.NodeInfo{
			Node:      node,
			Time:      nodeTime,
			RoomCount: len(nodeRooms[node]),
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Node < nodes[j].Node
	})

	buildingNames := make([]string, 0, len(buildingMap))
	for buildingName := range buildingMap {
		buildingNames = append(buildingNames, buildingName)
	}
	sort.Slice(buildingNames, func(i, j int) bool {
		if len(buildingNames[i]) != len(buildingNames[j]) {
			return len(buildingNames[i]) < len(buildingNames[j])
		}
		return buildingNames[i] < buildingNames[j]
	})

	buildings := make([]model.BuildingInfo, 0, len(buildingNames))
	for _, buildingName := range buildingNames {
		roomMap := buildingMap[buildingName]
		rooms := make([]model.RoomInfo, 0, len(roomMap))
		for _, room := range roomMap {
			sort.Ints(room.FreeNodes)
			room.FreeNodes = uniqueInts(room.FreeNodes)
			sort.Slice(room.FreeTimes, func(i, j int) bool {
				if room.FreeTimes[i].Node != room.FreeTimes[j].Node {
					return room.FreeTimes[i].Node < room.FreeTimes[j].Node
				}
				return room.FreeTimes[i].Time < room.FreeTimes[j].Time
			})
			room.FreeTimes = uniqueFreeTimes(room.FreeTimes)
			rooms = append(rooms, *room)
		}
		sort.Slice(rooms, func(i, j int) bool {
			return rooms[i].DisplayName < rooms[j].DisplayName
		})
		buildings = append(buildings, model.BuildingInfo{
			Name:  buildingName,
			Rooms: rooms,
		})
	}

	return model.CampusInfo{
		ID:        campusConfig.ID,
		Name:      campusConfig.Name,
		Buildings: buildings,
		Nodes:     nodes,
	}
}

func parseRoom(raw string) (string, string, string, int, bool) {
	raw = strings.TrimSpace(raw)
	matches := roomPattern.FindStringSubmatch(raw)
	if len(matches) != 3 {
		return "未分组", raw, raw, 0, raw != ""
	}
	capacity, _ := strconv.Atoi(matches[2])
	buildingName, roomName := splitRoomName(matches[1])
	return buildingName, roomName, buildingName + "-" + roomName, capacity, true
}

func uniqueInts(values []int) []int {
	if len(values) < 2 {
		return values
	}
	write := 1
	for read := 1; read < len(values); read++ {
		if values[read] == values[read-1] {
			continue
		}
		values[write] = values[read]
		write++
	}
	return values[:write]
}

func uniqueFreeTimes(values []model.FreeTime) []model.FreeTime {
	if len(values) < 2 {
		return values
	}
	write := 1
	for read := 1; read < len(values); read++ {
		if values[read].Node == values[read-1].Node && values[read].Time == values[read-1].Time {
			continue
		}
		values[write] = values[read]
		write++
	}
	return values[:write]
}

func splitRoomName(name string) (string, string) {
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return "未分组", name
	}
	if _, err := strconv.Atoi(parts[len(parts)-1]); err == nil && len(parts) > 2 {
		return strings.Join(parts[:len(parts)-2], "-"), strings.Join(parts[len(parts)-2:], "-")
	}
	return strings.Join(parts[:len(parts)-1], "-"), parts[len(parts)-1]
}

func encryptJWPassword(password string) (string, error) {
	plainJSON, err := json.Marshal(password)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher([]byte(tokenPasswordKey))
	if err != nil {
		return "", err
	}

	padded := pkcs7Pad(plainJSON, block.BlockSize())
	encrypted := make([]byte, len(padded))
	for start := 0; start < len(padded); start += block.BlockSize() {
		block.Encrypt(encrypted[start:start+block.BlockSize()], padded[start:start+block.BlockSize()])
	}
	firstBase64 := base64.StdEncoding.EncodeToString(encrypted)
	return base64.StdEncoding.EncodeToString([]byte(firstBase64)), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+padding)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(padding)
	}
	return out
}

func getCachedTodayClassrooms() (*model.TodayClassrooms, bool) {
	raw, ok := cache.GetCache(TodayCacheKey)
	if !ok || raw == nil {
		return nil, false
	}
	cached, ok := raw.(*model.TodayClassrooms)
	if !ok || cached == nil {
		return nil, false
	}
	if cached.Date != time.Now().Format("2006-01-02") {
		return nil, false
	}
	return cached, true
}

func cloneTodayClassrooms(in *model.TodayClassrooms) *model.TodayClassrooms {
	if in == nil {
		return nil
	}
	out := *in
	out.Campuses = make([]model.CampusInfo, len(in.Campuses))
	for i, campus := range in.Campuses {
		out.Campuses[i] = campus
		out.Campuses[i].Nodes = append([]model.NodeInfo(nil), campus.Nodes...)
		out.Campuses[i].Buildings = make([]model.BuildingInfo, len(campus.Buildings))
		for j, building := range campus.Buildings {
			out.Campuses[i].Buildings[j] = building
			out.Campuses[i].Buildings[j].Rooms = make([]model.RoomInfo, len(building.Rooms))
			for k, room := range building.Rooms {
				out.Campuses[i].Buildings[j].Rooms[k] = room
				out.Campuses[i].Buildings[j].Rooms[k].FreeNodes = append([]int(nil), room.FreeNodes...)
				out.Campuses[i].Buildings[j].Rooms[k].FreeTimes = append([]model.FreeTime(nil), room.FreeTimes...)
			}
		}
	}
	if in.Error != nil {
		errCopy := *in.Error
		out.Error = &errCopy
	}
	return &out
}

func endOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 23, 59, 59, 0, t.Location())
}

func joinAPIPath(apiURL string, path string) (string, error) {
	parsed, err := url.Parse(apiURL)
	if err != nil {
		return "", err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(path, "/")
	return parsed.String(), nil
}

func addQuery(rawURL string, values map[string]string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	for key, value := range values {
		query.Set(key, value)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func classifyError(err error) string {
	if err == nil {
		return ""
	}
	var jwErr *jwError
	if errors.As(err, &jwErr) {
		return string(jwErr.kind)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return string(jwErrorUpstream)
	}
	return string(jwErrorUpstream)
}

func SafeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	switch classifyError(err) {
	case string(jwErrorConfig):
		return "服务配置不完整，请检查教务系统凭据"
	case string(jwErrorAuth), string(jwErrorLogin):
		return "教务系统登录失败，请检查服务配置或稍后重试"
	case string(jwErrorQuery), string(jwErrorParse):
		return "教务系统查询失败，请稍后重试"
	default:
		return "数据获取失败，请稍后重试"
	}
}

func safeRemoteMessage(message string) string {
	if message == "" {
		return "remote service returned failure"
	}
	return strings.TrimSpace(message)
}

func recordLoginSuccess(at time.Time) {
	runtimeStatusMu.Lock()
	defer runtimeStatusMu.Unlock()
	runtimeStatus.LastLoginSuccessAt = cloneTime(at)
	runtimeStatus.LastLoginError = ""
}

func recordLoginFailure(err error) {
	runtimeStatusMu.Lock()
	defer runtimeStatusMu.Unlock()
	runtimeStatus.LastLoginError = SafeErrorMessage(err)
}

func recordRefreshSuccess(at time.Time) {
	runtimeStatusMu.Lock()
	defer runtimeStatusMu.Unlock()
	runtimeStatus.LastRefreshSuccessAt = cloneTime(at)
	runtimeStatus.LastRefreshError = ""
}

func recordRefreshFailure(err error) {
	runtimeStatusMu.Lock()
	defer runtimeStatusMu.Unlock()
	runtimeStatus.LastRefreshError = SafeErrorMessage(err)
}

func GetRuntimeStatus() RuntimeStatus {
	status := snapshotRuntimeStatus()
	now := time.Now()
	if cached, ok := getCachedTodayClassrooms(); ok {
		status.CacheAvailable = true
		status.CacheFresh = !cached.ExpiresAt.Before(now)
		status.CacheStale = now.Before(cached.StaleUntil)
		status.CacheDate = cached.Date
	}
	return status
}

func snapshotRuntimeStatus() RuntimeStatus {
	runtimeStatusMu.RLock()
	defer runtimeStatusMu.RUnlock()
	status := runtimeStatus
	if runtimeStatus.LastLoginSuccessAt != nil {
		status.LastLoginSuccessAt = cloneTime(*runtimeStatus.LastLoginSuccessAt)
	}
	if runtimeStatus.LastRefreshSuccessAt != nil {
		status.LastRefreshSuccessAt = cloneTime(*runtimeStatus.LastRefreshSuccessAt)
	}
	return status
}

func cloneTime(t time.Time) *time.Time {
	copy := t
	return &copy
}

func isJWErrorKind(err error, kind jwErrorKind) bool {
	var jwErr *jwError
	if !errors.As(err, &jwErr) {
		return false
	}
	return jwErr.kind == kind
}

func isAuthFailureMessage(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}
	for _, marker := range []string{"token", "login", "auth", "unauthorized", "forbidden", "登录", "认证", "授权", "过期", "失效"} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func validateJWAPIURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", newJWError(jwErrorConfig, "serverconfig", err, "invalid API URL")
	}
	if parsed.Scheme != "https" {
		return "", newJWError(jwErrorConfig, "serverconfig", nil, "API URL must use HTTPS")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", newJWError(jwErrorConfig, "serverconfig", nil, "API URL host is empty")
	}
	if host != "jwglweixin.bupt.edu.cn" && !strings.HasSuffix(host, ".bupt.edu.cn") {
		return "", newJWError(jwErrorConfig, "serverconfig", nil, "API URL host is not allowed")
	}
	if parsed.User != nil {
		return "", newJWError(jwErrorConfig, "serverconfig", nil, "API URL must not contain user info")
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return "", newJWError(jwErrorConfig, "serverconfig", nil, "API URL port is not allowed")
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}
