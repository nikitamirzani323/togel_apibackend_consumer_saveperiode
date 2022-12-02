package helpers

type Response struct {
	Status  int         `json:"status"`
	Message string      `json:"message"`
	Record  interface{} `json:"record"`
	Time    string      `json:"time"`
}

type ErrorResponse struct {
	Field string
	Tag   string
}

func ErrorCheck(err error) {
	if err != nil {
		panic(err.Error())
	}
}
