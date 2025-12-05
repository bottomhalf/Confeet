package model

type Message struct {
	ChannelId string `json:"channelId"`
	SenderId  string `json:"senderId"`
	Type      string `json:"type"`
	Content   string `json:"content"`
}
