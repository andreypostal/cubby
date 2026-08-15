package core

import "time"

// CORSConfig holds CORS policy settings
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins" env:"CUBBY_API_CORS_ALLOWED_ORIGINS" env-default:"*"`
	AllowedMethods []string `yaml:"allowed_methods" env:"CUBBY_API_CORS_ALLOWED_METHODS" env-default:"get,post,put,patch,delete,options"`
	AllowedHeaders []string `yaml:"allowed_headers" env:"CUBBY_API_CORS_ALLOWED_HEADERS" env-default:"Origin,Content-Type,Accept,Authorization"`
	ExposedHeaders []string `yaml:"exposed_headers" env:"CUBBY_API_CORS_EXPOSED_HEADERS" env-default:"Content-Disposition"`
	MaxAge         int      `yaml:"max_age" env:"CUBBY_API_CORS_MAX_AGE" env-default:"3600"`
}

type TLSConfig struct {
	CertFile string `yaml:"cert_file" env:"CUBBY_API_TLS_CERT_FILE"`
	KeyFile  string `yaml:"key_file" env:"CUBBY_API_TLS_KEY_FILE"`
}

type ApiConfig struct {
	Host string     `yaml:"host" env:"CUBBY_API_BIND_IP" env-default:"127.0.0.1"`
	Port int        `yaml:"port" env:"CUBBY_API_PORT" env-default:"8080"`
	CORS CORSConfig `yaml:"cors"`
	TLS  TLSConfig  `yaml:"tls"`

	ReadTimeout  time.Duration `yaml:"read_timeout" env:"CUBBY_API_READ_TIMEOUT" env-default:"10s"`
	WriteTimeout time.Duration `yaml:"write_timeout" env:"CUBBY_API_WRITE_TIMEOUT" env-default:"60s"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" env:"CUBBY_API_IDLE_TIMEOUT" env-default:"60s"`
}

type DBConfig struct {
	SqliteParams    string        `yaml:"sqlite_params" env:"CUBBY_DB_SQLITE_PARAMS"`
	MaxOpenConns    int           `yaml:"max_open_conns" env:"CUBBY_DB_MAX_OPEN_CONNS" env-default:"4"`
	MaxIdleConns    int           `yaml:"max_idle_conns" env:"CUBBY_DB_MAX_IDLE_CONNS" env-default:"2"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env:"CUBBY_DB_CONN_MAX_LIFETIME" env-default:"1h"`
}

// JWTConfig holds JWT signing and expiration settings
type JWTConfig struct {
	// Secret signs the session tokens. Left empty the server generates one into
	// the data folder on first boot, so there is no default to leak.
	Secret    string        `yaml:"secret" env:"CUBBY_JWT_SECRET"`
	ExpiresIn time.Duration `yaml:"expires_in" env:"CUBBY_JWT_EXPIRES_IN" env-default:"24h"`
}

// IdxConfig holds settings for search indexer
type IdxConfig struct {
	WriterPoolSize int           `yaml:"writer_pool_size" env:"CUBBY_SHOP_INDEXER_WRITER_POOL_SIZE" env-default:"5"`
	ReaderPoolSize int           `yaml:"reader_pool_size" env:"CUBBY_SHOP_INDEXER_READER_POOL_SIZE" env-default:"5"`
	WriterIdleTime time.Duration `yaml:"writer_idle_time" env:"CUBBY_SHOP_INDEXER_WRITER_IDLE_TIME" env-default:"5m"`
	ReaderIdleTime time.Duration `yaml:"reader_idle_time" env:"CUBBY_SHOP_INDEXER_READER_IDLE_TIME" env-default:"10m"`

	SuggestionLimit int           `yaml:"suggestion_limit" env:"CUBBY_SHOP_INDEXER_SUGGESTION_LIMIT" env-default:"5"`
	SuggestTimeout  time.Duration `yaml:"suggest_timeout" env:"CUBBY_SHOP_INDEXER_SUGGEST_TIMEOUT" env-default:"5s"`
}

// WebConfig holds settings for the embedded web UI module
type WebConfig struct {
	Enabled bool `yaml:"enabled" env:"CUBBY_WEB_ENABLED" env-default:"true"`
}

// LogConfig holds how the server writes its logs
type LogConfig struct {
	Level  string `yaml:"level" env:"CUBBY_LOG_LEVEL" env-default:"info"`
	Format string `yaml:"format" env:"CUBBY_LOG_FORMAT" env-default:"auto"`
}

type Config struct {
	Env       string `yaml:"env" env:"CUBBY_ENV" env-default:"dev"`
	DataDir   string `yaml:"data_dir" env:"CUBBY_DATA_DIR" env-default:"data"`
	BackupDir string `yaml:"backup_dir" env:"CUBBY_BACKUP_DIR" env-default:"backup"`
	// BackupKeep is how many backup archives are retained in BackupDir; older
	// ones are removed after a new backup is generated. Zero or less disables it.
	BackupKeep int       `yaml:"backup_keep" env:"CUBBY_BACKUP_KEEP" env-default:"5"`
	API        ApiConfig `yaml:"api"`
	DB         DBConfig  `yaml:"db"`
	JWT        JWTConfig `yaml:"jwt"`
	Indexer    IdxConfig `yaml:"indexer"`
	Web        WebConfig `yaml:"web"`
	Log        LogConfig `yaml:"log"`
}
