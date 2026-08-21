package service

import (
	"testing"

	"BUPT_EC/config"
	"BUPT_EC/service/model"
)

func TestParseRoom(t *testing.T) {
	building, room, displayName, capacity, ok := parseRoom("教学实验综合楼-N104(229)")
	if !ok {
		t.Fatal("expected room to parse")
	}
	if building != "教学实验综合楼" || room != "N104" || displayName != "教学实验综合楼-N104" || capacity != 229 {
		t.Fatalf("unexpected parsed room: %q %q %q %d", building, room, displayName, capacity)
	}

	building, room, displayName, capacity, ok = parseRoom("未来学习大楼-202-203(60)")
	if !ok {
		t.Fatal("expected merged room to parse")
	}
	if building != "未来学习大楼" || room != "202-203" || displayName != "未来学习大楼-202-203" || capacity != 60 {
		t.Fatalf("unexpected parsed merged room: %q %q %q %d", building, room, displayName, capacity)
	}

	building, room, displayName, capacity, ok = parseRoom("教学实验综合楼-N104（229）")
	if !ok {
		t.Fatal("expected full-width parentheses room to parse")
	}
	if building != "教学实验综合楼" || room != "N104" || displayName != "教学实验综合楼-N104" || capacity != 229 {
		t.Fatalf("unexpected parsed full-width room: %q %q %q %d", building, room, displayName, capacity)
	}
}

func TestBuildCampusInfoDeduplicatesRooms(t *testing.T) {
	campus := buildCampusInfo(config.CampusConfig{ID: "01", Name: "西土城"}, []model.JWClassInfo{
		{
			NodeName:   "1",
			NodeTime:   "08:00-08:45",
			Classrooms: "教学实验综合楼-N104(229),教学实验综合楼-N104(229)",
		},
	})

	if len(campus.Nodes) != 1 {
		t.Fatalf("expected one node, got %d", len(campus.Nodes))
	}
	if campus.Nodes[0].RoomCount != 1 {
		t.Fatalf("expected deduplicated room count 1, got %d", campus.Nodes[0].RoomCount)
	}
	if len(campus.Buildings) != 1 || len(campus.Buildings[0].Rooms) != 1 {
		t.Fatalf("expected one deduplicated room, got %#v", campus.Buildings)
	}
	room := campus.Buildings[0].Rooms[0]
	if len(room.FreeNodes) != 1 || room.FreeNodes[0] != 1 {
		t.Fatalf("expected one deduplicated free node, got %#v", room.FreeNodes)
	}
	if len(room.FreeTimes) != 1 || room.FreeTimes[0].Node != 1 {
		t.Fatalf("expected one deduplicated free time, got %#v", room.FreeTimes)
	}
}

func TestBuildingDisplayNameNormalization(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"未来学习大楼", "主楼"}, // alias table (F-07)
		{"1", "教1"},      // numeric prefix rule
		{"12", "教12"},
		{"教学实验综合楼", "教学实验综合楼"}, // passthrough
	}
	for _, tc := range cases {
		if got := buildingDisplayName(tc.raw); got != tc.want {
			t.Errorf("buildingDisplayName(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestBuildCampusInfoEmitsBuildingDisplayName(t *testing.T) {
	campus := buildCampusInfo(config.CampusConfig{ID: "04", Name: "沙河"}, []model.JWClassInfo{
		{
			NodeName:   "1",
			NodeTime:   "08:00-08:45",
			Classrooms: "未来学习大楼-N104(229),1-101(40)",
		},
	})

	want := map[string]string{
		"未来学习大楼": "主楼",
		"1":      "教1",
	}
	if len(campus.Buildings) != len(want) {
		t.Fatalf("expected %d buildings, got %#v", len(want), campus.Buildings)
	}
	for _, b := range campus.Buildings {
		if want[b.Name] != b.DisplayName {
			t.Errorf("building %q display_name = %q, want %q", b.Name, b.DisplayName, want[b.Name])
		}
	}
}
