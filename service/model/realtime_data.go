package model

import "time"

type ServerConfigResponse struct {
	APIURL     string `json:"ApiUrl"`
	Title      string `json:"title"`
	SelectURL  string `json:"SelectUrl"`
	SchoolCode string `json:"schoolCode"`
}

type LoginResponse struct {
	Code string `json:"code"`
	Msg  string `json:"Msg"`
	Data struct {
		Token string `json:"token"`
	} `json:"data"`
}

type JWClassInfo struct {
	Classrooms string `json:"CLASSROOMS"`
	NodeTime   string `json:"NODETIME"`
	NodeName   string `json:"NODENAME"`
}

type QueryResponse struct {
	Code string        `json:"code"`
	Msg  string        `json:"Msg"`
	Data []JWClassInfo `json:"data"`
}

type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type NodeInfo struct {
	Node      int    `json:"node"`
	Time      string `json:"time"`
	RoomCount int    `json:"room_count"`
}

type FreeTime struct {
	Node int    `json:"node"`
	Time string `json:"time"`
}

type RoomInfo struct {
	Name        string     `json:"name"`
	DisplayName string     `json:"display_name"`
	Capacity    int        `json:"capacity"`
	FreeNodes   []int      `json:"free_nodes"`
	FreeTimes   []FreeTime `json:"free_times"`
}

type BuildingInfo struct {
	Name  string     `json:"name"`
	Rooms []RoomInfo `json:"rooms"`
}

type CampusInfo struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Buildings []BuildingInfo `json:"buildings"`
	Nodes     []NodeInfo     `json:"nodes"`
}

type TodayClassrooms struct {
	Date            string       `json:"date"`
	UpdatedAt       time.Time    `json:"updated_at"`
	ExpiresAt       time.Time    `json:"expires_at"`
	StaleUntil      time.Time    `json:"stale_until"`
	Stale           bool         `json:"stale"`
	Campuses        []CampusInfo `json:"campuses"`
	PartialCampuses []string     `json:"partial_campuses,omitempty"`
	Error           *APIError    `json:"error"`
}
