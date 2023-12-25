package arkose

var (
	KeyMap = map[string]int{
		"0A1D34FC-659D-4E23-B17B-694DCFCF6A6C": 0, //auth
		"3D86FBBA-9D22-402A-B512-3420086BA6CC": 1, //chat3
		"35536E1E-65B4-4D96-9D97-6ADB7EFF8147": 2, //chat4
	}
)

type KvPair struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
