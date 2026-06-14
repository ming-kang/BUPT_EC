package service

import (
	"EmptyClassroom/cache"
	"EmptyClassroom/config"
	"EmptyClassroom/logs"
	"EmptyClassroom/service/model"
	"EmptyClassroom/utils"
	"context"
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	ServerConfigURL = "https://jwglweixin.bupt.edu.cn/sjd/serverconfig.json"
	DefaultAPIURL   = "http://jwglweixin.bupt.edu.cn/bjyddx/"

	LoginUsernameKey = "JW_USERNAME"
	LoginPasswordKey = "JW_PASSWORD"
	LoginTokenKey    = "JW_TOKEN"

	TodayCacheKey = "TODAY_CLASSROOMS_CACHE"

	tokenPasswordKey = "qzkj1kjghd=876&*"
)

var (
	ErrNoTodayCache = errors.New("no today classroom cache")

	tokenManager = &TokenManager{}

	roomPattern = regexp.MustCompile(`^(.+)[(（](\d+)[)）]$`)
)

type TokenManager struct {
	mu     sync.Mutex
	token  string
	apiURL string
}

func ResetRuntimeStateForTest() {
	tokenManager = &TokenManager{}
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

func GetData(ctx context.Context, c *gin.Context) {
	todayData, err := GetTodayClassrooms(ctx)
	if err != nil {
		c.JSON(500, gin.H{
			"code": 500,
			"msg":  safeErrorMessage(err),
			"data": nil,
		})
		return
	}
	c.JSON(200, gin.H{
		"code": 0,
		"data": todayData,
	})
}

func GetTodayClassrooms(ctx context.Context) (*model.TodayClassrooms, error) {
	if cached, ok := getCachedTodayClassrooms(); ok && !cached.ExpiresAt.Before(time.Now()) {
		resp := cloneTodayClassrooms(cached)
		resp.Stale = false
		resp.Error = nil
		return resp, nil
	}

	fresh, err := refreshTodayClassrooms(ctx)
	if err == nil {
		return fresh, nil
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
	now := time.Now()
	today := &model.TodayClassrooms{
		Date:       now.Format("2006-01-02"),
		UpdatedAt:  now,
		ExpiresAt:  now.Add(5 * time.Minute),
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

	token, refreshErr := tokenManager.EnsureToken(ctx, true)
	if refreshErr != nil {
		return nil, refreshErr
	}
	rows, retryErr := queryCampusWithToken(ctx, campusID, token)
	if retryErr != nil {
		return nil, retryErr
	}
	return rows, nil
}

func queryCampusWithToken(ctx context.Context, campusID string, token string) ([]model.JWClassInfo, error) {
	apiURL, err := tokenManager.APIURL(ctx)
	if err != nil {
		return nil, err
	}
	queryURL, err := joinAPIPath(apiURL, "todayClassrooms")
	if err != nil {
		return nil, err
	}
	queryURL = addQuery(queryURL, map[string]string{"campusId": campusID})

	code, _, body, err := utils.HttpPostWithHeader(ctx, queryURL, map[string]string{"token": token})
	if err != nil {
		return nil, fmt.Errorf("jw query request failed: %w", err)
	}
	if code != 200 {
		return nil, fmt.Errorf("jw query http status %d", code)
	}

	var resp model.QueryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("jw query response invalid: %w", err)
	}
	if resp.Code != "1" {
		return nil, fmt.Errorf("jw query failed: %s", safeRemoteMessage(resp.Msg))
	}
	return resp.Data, nil
}

func (m *TokenManager) EnsureToken(ctx context.Context, forceRefresh bool) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !forceRefresh && m.token != "" {
		return m.token, nil
	}
	if token := os.Getenv(LoginTokenKey); token != "" && !forceRefresh {
		m.token = token
		return m.token, nil
	}

	apiURL, err := m.apiURLLocked(ctx)
	if err != nil {
		return "", err
	}
	loginURL, err := joinAPIPath(apiURL, "login")
	if err != nil {
		return "", err
	}

	userNo := os.Getenv(LoginUsernameKey)
	password := os.Getenv(LoginPasswordKey)
	if userNo == "" || password == "" {
		return "", errors.New("JW_USERNAME or JW_PASSWORD is not configured")
	}

	encryptedPassword, err := encryptJWPassword(password)
	if err != nil {
		return "", fmt.Errorf("encrypt login password failed: %w", err)
	}
	req := map[string]string{
		"userNo":      userNo,
		"pwd":         encryptedPassword,
		"encode":      "1",
		"captchaData": "",
		"codeVal":     "",
	}

	code, _, body, err := utils.HttpPostForm(ctx, loginURL, req)
	if err != nil {
		return "", fmt.Errorf("jw login request failed: %w", err)
	}
	if code != 200 {
		return "", fmt.Errorf("jw login http status %d", code)
	}

	var resp model.LoginResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("jw login response invalid: %w", err)
	}
	if resp.Code != "1" || resp.Data.Token == "" {
		return "", fmt.Errorf("jw login failed: %s", safeRemoteMessage(resp.Msg))
	}

	m.token = resp.Data.Token
	return m.token, nil
}

func (m *TokenManager) APIURL(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.apiURLLocked(ctx)
}

func (m *TokenManager) apiURLLocked(ctx context.Context) (string, error) {
	if m.apiURL != "" {
		return m.apiURL, nil
	}

	code, _, body, err := utils.HttpGet(ctx, ServerConfigURL)
	if err != nil {
		logs.CtxWarn(ctx, "serverconfig request failed; using default API URL: %v", err)
		m.apiURL = DefaultAPIURL
		return m.apiURL, nil
	}
	if code != 200 {
		logs.CtxWarn(ctx, "serverconfig http status %d; using default API URL", code)
		m.apiURL = DefaultAPIURL
		return m.apiURL, nil
	}

	var resp model.ServerConfigResponse
	if err := json.Unmarshal(body, &resp); err != nil || resp.APIURL == "" {
		logs.CtxWarn(ctx, "serverconfig invalid; using default API URL")
		m.apiURL = DefaultAPIURL
		return m.apiURL, nil
	}
	m.apiURL = resp.APIURL
	return m.apiURL, nil
}

func buildCampusInfo(campusConfig config.CampusConfig, rows []model.JWClassInfo) model.CampusInfo {
	buildingMap := map[string]map[string]*model.RoomInfo{}
	nodeRoomCounts := map[int]int{}
	nodeTimes := map[int]string{}

	for _, row := range rows {
		node, err := strconv.Atoi(row.NodeName)
		if err != nil {
			continue
		}
		nodeTimes[node] = row.NodeTime
		classrooms := strings.Split(row.Classrooms, ",")
		nodeRoomCounts[node] = len(classrooms)
		for _, raw := range classrooms {
			buildingName, roomName, displayName, capacity, ok := parseRoom(raw)
			if !ok {
				continue
			}
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
			RoomCount: nodeRoomCounts[node],
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
			sort.Slice(room.FreeTimes, func(i, j int) bool {
				return room.FreeTimes[i].Node < room.FreeTimes[j].Node
			})
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
	msg := err.Error()
	switch {
	case strings.Contains(msg, "login"):
		return "jw_login_failed"
	case strings.Contains(msg, "query"):
		return "jw_query_failed"
	default:
		return "jw_unavailable"
	}
}

func safeErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	switch classifyError(err) {
	case "jw_login_failed":
		return "教务系统登录失败，请检查服务配置或稍后重试"
	case "jw_query_failed":
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
