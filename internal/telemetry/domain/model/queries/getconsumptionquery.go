package queries

import "time"

type GetConsumptionQuery struct {
	UnitID, Token string
	From, To      time.Time
	Limit         int
}
