package model

type Residence struct {
	ID           int64
	CampusID     int64
	Code         string
	Name         string
	GenderPolicy string
}

type DormUnit struct {
	ID         int64
	FloorID    int64
	UnitNumber string
	Code       string
	UnitType   string
	Capacity   int
}

type DormUnitFilter struct {
	ResidenceID *int64
	FloorID     *int64
	UnitType    *string
}
