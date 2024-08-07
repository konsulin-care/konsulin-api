package config

type DriverConfig struct {
	MongoDB  MongoDB  `mapstructure:"mongodb"`
	Redis    Redis    `mapstructure:"redis"`
	Logger   Logger   `mapstructure:"logger"`
	RabbitMQ RabbitMQ `mapstructure:"rabbitmq"`
	Minio    Minio    `mapstructure:"minio"`
}

type MongoDB struct {
	Port     string `mapstructure:"port"`
	Host     string `mapstructure:"host"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type Redis struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
}

type Logger struct {
	Level               string `mapstructure:"level"`
	OutputFileName      string `mapstructure:"output_file_name"`
	OutputErrorFileName string `mapstructure:"output_error_file_name"`
}

type RabbitMQ struct {
	Port     string `mapstructure:"port"`
	Host     string `mapstructure:"host"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type Minio struct {
	Port     string `mapstructure:"port"`
	Host     string `mapstructure:"host"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	UseSSL   bool   `mapstructure:"use_ssl"`
}
