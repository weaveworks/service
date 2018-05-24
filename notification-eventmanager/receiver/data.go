package receiver

type Data interface {
	Type() string
}

type UnknownData struct{
	Text string `json:"text"`

}

func (u UnknownData) Type() string {
	return "unknown"
}
