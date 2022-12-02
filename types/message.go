package types

type Item struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Method string `json:"action"`
}

type ActionType string

// AddI(), DeleteI(), GetI(), GetAllI()
const (
	GetAllI = "GetAllItems"
	GetI    = "GetItem"
	AddI    = "AddItem"
	DeleteI = "RemoveItem"
)
