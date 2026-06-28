package idcodec

// Interface is the public ID codec contract.
type Interface interface {
	Encode(prefix string, id int64) (string, error)
	Decode(value string) (string, int64, error)
	DecodeWithPrefix(prefix string, value string) (int64, error)
}
