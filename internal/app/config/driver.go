package config

type DriverConfig struct {
	Redis      Redis      `mapstructure:"redis"`
	Logger     Logger     `mapstructure:"logger"`
	RabbitMQ   RabbitMQ   `mapstructure:"rabbitmq"`
	Supertoken Supertoken `mapstructure:"supertoken"`
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

type Supertoken struct {
	ApiBasePath     string `mapstructure:"api_base_path"`
	WebsiteBasePath string `mapstructure:"website_base_path"`
	ConnectionURI   string `mapstructure:"connection_uri"`
	APIKey          string `mapstructure:"api_key"`
	AppName         string `mapstructure:"app_name"`
	ApiDomain       string `mapstructure:"api_domain"`
	WebsiteDomain   string `mapstructure:"website_domain"`
}
