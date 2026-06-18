package vault

// Config holds the vault configuration.
type Config struct {
	MasterKeyPath string          `yaml:"master_key_path" json:"master_key_path"`
	Encrypted     []EncryptedRoom `yaml:"encrypted"       json:"encrypted"`
	Honeypots     []Honeypot      `yaml:"honeypots"       json:"honeypots"`
	Mirrors       []Mirror        `yaml:"mirrors"         json:"mirrors"`
}

// EncryptedRoom marks a wing/room pair for encryption.
type EncryptedRoom struct {
	Room string `yaml:"room" json:"room"`
	Wing string `yaml:"wing" json:"wing"` // empty = all wings
}

// Honeypot defines a fake room that triggers alerts on access.
type Honeypot struct {
	Room string `yaml:"room" json:"room"`
	Wing string `yaml:"wing" json:"wing"` // empty = all wings
	On   string `yaml:"on"   json:"on"`   // "read" or "any"
}

// Mirror defines a room that returns fake data for unauthorized access.
type Mirror struct {
	Room      string `yaml:"room"       json:"room"`
	Wing      string `yaml:"wing"       json:"wing"` // empty = all wings
	MatchRole string `yaml:"match_role" json:"match_role"` // "all", "unauthorized", "authorized"
	Fake      string `yaml:"fake"       json:"fake"`
}
