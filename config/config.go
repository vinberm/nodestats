package config

type Config struct {
	RootDir          string `mapstructure:"home"`
	ListenAddress    string `mapstructure:"laddr"`
	Seeds            string `mapstructure:"seeds"`
	SkipUPNP         bool   `mapstructure:"skip_upnp"`
	AddrBook         string `mapstructure:"addr_book_file"`

	PexReactor       bool   `mapstructure:"pex"`
	MaxNumPeers      int    `mapstructure:"max_num_peers"`
	HandshakeTimeout int    `mapstructure:"handshake_timeout"`
	DialTimeout      int    `mapstructure:"dial_timeout"`
}


func DefaultConfig() *Config {
	return &Config{
		ListenAddress:    "tcp://0.0.0.0:46656",
		AddrBook:         "addrbook.json",
		MaxNumPeers:      50,
		HandshakeTimeout: 30,
		DialTimeout:      3,
	}
}
