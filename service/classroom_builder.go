package service

import (
	"BUPT_EC/config"
	"BUPT_EC/service/model"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var roomPattern = regexp.MustCompile(`^(.+)[(（](\d+)[)）]$`)

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
