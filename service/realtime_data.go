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

	"golang.org/x/sync/errgroup"
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
	staleRefreshWait      = 300 * time.Millisecond
	staleRefreshBackoff   = 30 * time.Second
)

var (
	ErrNoTodayCache = errors.New("no today classroom cache")

	tokenManager = &TokenManager{}

	roomPattern = regexp.MustCompile(`^(.+)[(（](\d+)[)）]$`)

	nowFunc                  = time.Now
	queryCampusForRefresh    = queryCampus
	queryCampusWithTokenFunc = queryCampusWithToken
	refreshTokenFunc         = (*TokenManager).refreshToken
)

var (
	refreshStateMu     sync.Mutex
	refreshInFlight    bool
	refreshAttempt     *classroomRefreshAttempt
	nextRefreshAllowed time.Time
	lastRefreshError   error
	refreshWorkers     sync.WaitGroup
)

type classroomRefreshAttempt struct {
	done   chan struct{}
	result classroomRefreshResult
}

type classroomRefreshResult struct {
	value *model.TodayClassrooms
	err   error
}

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

type jwResponseEnvelope struct {
	Code string          `json:"code"`
	Msg  string          `json:"Msg"`
	Data json.RawMessage `json:"data"`
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
	refreshWorkers.Wait()
	tokenManager = &TokenManager{}
	resetRefreshState()
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

func StartClassroomWarmup() {
	go func() {
		attempt, started := startClassroomRefresh(nowFunc())
		if !started {
			return
		}
		<-attempt.done
	}()
}

func GetTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error) {
	now := nowFunc()
	if cached, ok := getCachedTodayClassrooms(); ok {
		if !cached.ExpiresAt.Before(now) {
			return classroomResponse(cached, false, nil), nil
		}
		if now.Before(cached.StaleUntil) {
			return getStaleTodayClassrooms(ctx, cached, now), nil
		}
	}

	attempt, started := startClassroomRefresh(now)
	if !started {
		if err := getLastRefreshError(); err != nil {
			return nil, err
		}
		return nil, ErrNoTodayCache
	}
	select {
	case <-attempt.done:
		return classroomResponseFromRefresh(attempt.result)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func getStaleTodayClassrooms(ctx context.Context, cached *model.TodayClassrooms, now time.Time) *model.TodayClassrooms {
	attempt, started := startClassroomRefresh(now)
	if !started {
		return classroomResponse(cached, true, staleAPIError(getLastRefreshError()))
	}

	timer := time.NewTimer(staleRefreshWait)
	defer timer.Stop()

	select {
	case <-attempt.done:
		if attempt.result.err == nil {
			fresh, err := classroomResponseFromRefresh(attempt.result)
			if err == nil {
				return fresh
			}
			return classroomResponse(cached, true, staleAPIError(err))
		}
		return classroomResponse(cached, true, staleAPIError(attempt.result.err))
	case <-timer.C:
		return classroomResponse(cached, true, nil)
	case <-ctx.Done():
		return classroomResponse(cached, true, nil)
	}
}

func startClassroomRefresh(now time.Time) (*classroomRefreshAttempt, bool) {
	refreshStateMu.Lock()
	if refreshInFlight {
		attempt := refreshAttempt
		refreshStateMu.Unlock()
		return attempt, true
	}

	if !nextRefreshAllowed.IsZero() && now.Before(nextRefreshAllowed) {
		refreshStateMu.Unlock()
		return nil, false
	}

	refreshInFlight = true
	attempt := &classroomRefreshAttempt{done: make(chan struct{})}
	refreshAttempt = attempt
	refreshWorkers.Add(1)
	refreshStateMu.Unlock()

	go func() {
		defer refreshWorkers.Done()
		refreshCtx, cancel := context.WithTimeout(context.Background(), classroomRefreshLimit)
		defer cancel()

		today, err := refreshTodayClassrooms(refreshCtx)
		finishClassroomRefresh(attempt, classroomRefreshResult{value: today, err: err})
	}()
	return attempt, true
}

func finishClassroomRefresh(attempt *classroomRefreshAttempt, result classroomRefreshResult) {
	refreshStateMu.Lock()
	defer refreshStateMu.Unlock()

	attempt.result = result
	if refreshAttempt == attempt {
		refreshInFlight = false
		refreshAttempt = nil
	}
	if result.err != nil {
		lastRefreshError = result.err
		nextRefreshAllowed = nowFunc().Add(staleRefreshBackoff)
	} else {
		lastRefreshError = nil
		nextRefreshAllowed = time.Time{}
	}
	close(attempt.done)
}

func resetRefreshState() {
	refreshStateMu.Lock()
	defer refreshStateMu.Unlock()
	refreshInFlight = false
	refreshAttempt = nil
	nextRefreshAllowed = time.Time{}
	lastRefreshError = nil
}

func getLastRefreshError() error {
	refreshStateMu.Lock()
	defer refreshStateMu.Unlock()
	return lastRefreshError
}

func classroomResponseFromRefresh(result classroomRefreshResult) (*model.TodayClassrooms, error) {
	if result.err != nil {
		return nil, result.err
	}
	if result.value == nil {
		return nil, newJWError(jwErrorParse, "classroom refresh", nil, "unexpected refresh result")
	}
	return classroomResponse(result.value, false, nil), nil
}

func staleAPIError(err error) *model.APIError {
	if err == nil {
		return nil
	}
	return &model.APIError{
		Type:    classifyError(err),
		Message: "教务系统暂时不可用，当前展示的是今天最后一次成功刷新数据",
	}
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
	now := nowFunc()
	campuses := config.GetConfig().Campuses
	today := &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now,
		ExpiresAt:  now.Add(classroomFreshTTL),
		StaleUntil: endOfDay(now),
		Stale:      false,
		Campuses:   make([]model.CampusInfo, len(campuses)),
		Error:      nil,
	}

	group, groupCtx := errgroup.WithContext(ctx)
	for i, campusConfig := range campuses {
		i, campusConfig := i, campusConfig
		group.Go(func() error {
			jwRows, err := queryCampusForRefresh(groupCtx, campusConfig.ID, false)
			if err != nil {
				return err
			}
			today.Campuses[i] = buildCampusInfo(campusConfig, jwRows)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	cache.SetCache(TodayCacheKey, today, time.Until(today.StaleUntil))
	return classroomResponse(today, false, nil), nil
}

func queryCampus(ctx context.Context, campusID string, forceRefresh bool) ([]model.JWClassInfo, error) {
	token, err := tokenManager.EnsureToken(ctx, forceRefresh)
	if err != nil {
		return nil, err
	}

	rows, err := queryCampusWithTokenFunc(ctx, campusID, token)
	if err == nil {
		return rows, nil
	}
	if !isJWErrorKind(err, jwErrorAuth) {
		return nil, err
	}

	tokenManager.clearTokenIfCurrent(token)
	token, refreshErr := tokenManager.EnsureToken(ctx, true)
	if refreshErr != nil {
		return nil, errors.Join(err, refreshErr)
	}
	rows, retryErr := queryCampusWithTokenFunc(ctx, campusID, token)
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
		kind, message := classifyJWHTTPError(code, body)
		return nil, newJWError(kind, "jw query", nil, "%s", message)
	}

	return parseJWQueryResponse(body)
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

	value, err, _ := m.tokenGroup.Do("jw-token", func() (interface{}, error) {
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
		loginCtx, cancel := context.WithTimeout(context.Background(), jwRequestTimeout)
		defer cancel()
		token, err := refreshTokenFunc(m, loginCtx)
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
		apiCtx, cancel := context.WithTimeout(context.Background(), jwRequestTimeout)
		defer cancel()
		apiURL, err := m.fetchAPIURL(apiCtx)
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

func (m *TokenManager) clearTokenIfCurrent(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.token == token {
		m.token = ""
	}
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
	buildingMap := map[string]map[string]*roomAccumulator{}
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
				buildingMap[buildingName] = map[string]*roomAccumulator{}
			}
			room := buildingMap[buildingName][displayName]
			if room == nil {
				room = &roomAccumulator{
					Name:        roomName,
					DisplayName: displayName,
					Capacity:    capacity,
					FreeNodes:   map[int]struct{}{},
					FreeTimes:   map[model.FreeTime]struct{}{},
				}
				buildingMap[buildingName][displayName] = room
			}
			room.FreeNodes[node] = struct{}{}
			room.FreeTimes[model.FreeTime{Node: node, Time: row.NodeTime}] = struct{}{}
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
			rooms = append(rooms, room.toRoomInfo())
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

type roomAccumulator struct {
	Name        string
	DisplayName string
	Capacity    int
	FreeNodes   map[int]struct{}
	FreeTimes   map[model.FreeTime]struct{}
}

func (r *roomAccumulator) toRoomInfo() model.RoomInfo {
	freeNodes := make([]int, 0, len(r.FreeNodes))
	for node := range r.FreeNodes {
		freeNodes = append(freeNodes, node)
	}
	sort.Ints(freeNodes)

	freeTimes := make([]model.FreeTime, 0, len(r.FreeTimes))
	for freeTime := range r.FreeTimes {
		freeTimes = append(freeTimes, freeTime)
	}
	sort.Slice(freeTimes, func(i, j int) bool {
		if freeTimes[i].Node != freeTimes[j].Node {
			return freeTimes[i].Node < freeTimes[j].Node
		}
		return freeTimes[i].Time < freeTimes[j].Time
	})

	return model.RoomInfo{
		Name:        r.Name,
		DisplayName: r.DisplayName,
		Capacity:    r.Capacity,
		FreeNodes:   freeNodes,
		FreeTimes:   freeTimes,
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
	if cached.Date != nowFunc().Format("2006-01-02") {
		return nil, false
	}
	return cached, true
}

func classroomResponse(in *model.TodayClassrooms, stale bool, apiErr *model.APIError) *model.TodayClassrooms {
	if in == nil {
		return nil
	}
	out := *in
	out.Stale = stale
	if apiErr != nil {
		errCopy := *apiErr
		out.Error = &errCopy
	} else {
		out.Error = nil
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

func parseJWQueryResponse(body []byte) ([]model.JWClassInfo, error) {
	var envelope jwResponseEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, newJWError(jwErrorParse, "jw query", err, "response invalid")
	}
	if envelope.Code != "1" {
		kind := jwErrorQuery
		if isAuthFailureCode(envelope.Code) || isAuthFailureMessage(envelope.Msg) {
			kind = jwErrorAuth
		}
		return nil, newJWError(kind, "jw query", nil, "%s", safeRemoteMessage(envelope.Msg))
	}

	var rows []model.JWClassInfo
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return rows, nil
	}
	if err := json.Unmarshal(envelope.Data, &rows); err != nil {
		return nil, newJWError(jwErrorParse, "jw query", err, "response data invalid")
	}
	return rows, nil
}

func classifyJWHTTPError(statusCode int, body []byte) (jwErrorKind, string) {
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		return jwErrorAuth, fmt.Sprintf("http status %d", statusCode)
	}

	var envelope jwResponseEnvelope
	if len(body) > 0 && json.Unmarshal(body, &envelope) == nil {
		message := safeRemoteMessage(envelope.Msg)
		if isAuthFailureCode(envelope.Code) || isAuthFailureMessage(envelope.Msg) {
			return jwErrorAuth, message
		}
		if message != "" && message != "remote service returned failure" {
			return jwErrorQuery, fmt.Sprintf("http status %d: %s", statusCode, message)
		}
	}

	return jwErrorQuery, fmt.Sprintf("http status %d", statusCode)
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

func isAuthFailureCode(code string) bool {
	code = strings.TrimSpace(code)
	return code == "401" || code == "403"
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
	now := nowFunc()
	if cached, ok := getCachedTodayClassrooms(); ok {
		status.CacheAvailable = true
		status.CacheFresh = !cached.ExpiresAt.Before(now)
		status.CacheStale = now.Before(cached.StaleUntil)
		status.CacheDate = cached.Date
	}
	return status
}

func HasUsableTodayCache() bool {
	cached, ok := getCachedTodayClassrooms()
	return ok && nowFunc().Before(cached.StaleUntil)
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
