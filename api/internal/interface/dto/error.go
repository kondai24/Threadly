package dto

// ErrorResponseはAPIエラーの公開形式を表す。
type ErrorResponse struct {
	Error string `json:"error"`
}
