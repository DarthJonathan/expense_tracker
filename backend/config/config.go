package config

var Cfg Config

type Config struct {
	Server struct {
		Port         string `yaml:"port" env:"PORT" env-default:"8080"`
		LoggingLevel string `yaml:"logLevel" env:"LOG_LEVEL" env-default:"info"`
	} `yaml:"server" env-prefix:"SERVER_"`

	Database struct {
		URL string `yaml:"url" env:"DATABASE_URL"`
	} `yaml:"database" env-prefix:"DB_"`

	Cors struct {
		AllowOrigin string `yaml:"allowOrigin" env:"CORS_ALLOW_ORIGIN" env-default:"*"`
	} `yaml:"cors" env-prefix:"CORS_"`

	Auth struct {
		JWTSecret   string `yaml:"jwtSecret" env:"JWT_SECRET" env-default:"change-me-in-production"`
		TokenExpiry int    `yaml:"tokenExpiryHours" env:"JWT_TOKEN_EXPIRY_HOURS" env-default:"168"`
	} `yaml:"auth" env-prefix:"AUTH_"`
}
