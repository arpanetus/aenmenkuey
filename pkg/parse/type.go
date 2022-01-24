package parse

type DefaultResponse struct {
	Content string `json:"content"`
}

type Song struct {
	Title    string `json:"name"`
	Link     string `json:"url"`
	IsParsed bool   `json:"isParsed"`
	RawValue string `json:"rawValue"`
}

type ErrorLog struct {
	Song *Song  `json:"song"`
	Err  string `json:"err"`
}
