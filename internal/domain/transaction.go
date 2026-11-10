package domain

import (
	"time"
)

type TransactionType string

const (
	TypeIncome  TransactionType = "income"
	TypeExpense TransactionType = "expense"
)

type Category string

const (
	CatSessionFee  Category = "session_fee"
	CatMonthlyFee  Category = "monthly_fee"
	CatShuttleSale Category = "shuttle_sale"
	CatDrinks      Category = "drinks"
	CatOtherIncome Category = "other_income"

	CatCourtRent    Category = "court_rent"
	CatShuttles     Category = "shuttles"
	CatUtilities    Category = "utilities"
	CatStaff        Category = "staff"
	CatOtherExpense Category = "other_expense"
)

var categoryTypes = map[Category]TransactionType{
	CatSessionFee:   TypeIncome,
	CatMonthlyFee:   TypeIncome,
	CatShuttleSale:  TypeIncome,
	CatDrinks:       TypeIncome,
	CatOtherIncome:  TypeIncome,
	CatCourtRent:    TypeExpense,
	CatShuttles:     TypeExpense,
	CatUtilities:    TypeExpense,
	CatStaff:        TypeExpense,
	CatOtherExpense: TypeExpense,
}

// Type returns the transaction type the category belongs to.
func (c Category) Type() (TransactionType, bool) {
	t, ok := categoryTypes[c]
	return t, ok
}

// Transaction is one income or expense entry. Date is the Vietnam-local
// calendar day (YYYY-MM-DD) and decides which month it is reported in.
type Transaction struct {
	ID        int64
	SessionID *int64
	PlayerID  *int64
	Type      TransactionType
	Category  Category
	Amount    int64
	Note      string
	Date      string
	// Auto marks the session-fee income the app keeps in sync with the
	// players' paid flags.
	Auto      bool
	DeletedAt *time.Time
}

const DateLayout = "2006-01-02"

func ValidDate(s string) bool {
	_, err := time.Parse(DateLayout, s)
	return err == nil
}

func (t *Transaction) Validate() error {
	typ, ok := t.Category.Type()
	if !ok || typ != t.Type {
		return ErrInvalidCategory
	}
	if t.Amount < 0 {
		return ErrInvalidAmount
	}
	if !ValidDate(t.Date) {
		return ErrInvalidDate
	}
	return nil
}

var vnLocation = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)

// VNLocation is the display and reporting timezone. Vietnam has no DST, so
// a fixed offset avoids depending on the host's tzdata.
func VNLocation() *time.Location { return vnLocation }

// VNDate is the Vietnam-local calendar day of an instant.
func VNDate(t time.Time) string { return t.In(vnLocation).Format(DateLayout) }
