package emailw

type EmailConfig struct {
	Host     string `json:"host" yaml:"host"`
	Port     int    `json:"port" yaml:"port"`
	User     string `json:"user" yaml:"user"`
	Password string `json:"password" yaml:"password"`
	UsingSSL bool   `json:"using_t_ls"`
}

type SentEmailParam struct {
	Subject     string                 `json:"subject"`
	Sender      string                 `json:"sender"`
	To          []string               `json:"to"`
	Cc          []SentEmailParamCC     `json:"cc"`
	Message     string                 `json:"message"`
	MessageType string                 `json:"message_type"`
	Template    string                 `json:"template"`
	Param       map[string]interface{} `json:"param"`
	Attachments []SentEmailParamAttach `json:"attachments"`
}

type SentEmailParamAttach struct {
	FileName    string `json:"file_name"`
	FileBase64  string `json:"file_base64"`
	ContentType string `json:"content_type"`
}

type SentEmailParamCC struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}
