package alerts

type Kind string

const (
	Price   Kind = "price"
	Percent Kind = "percent"
)

type Direction string

const (
	GreaterOrEqual Direction = "gte"
	LessOrEqual    Direction = "lte"
)
