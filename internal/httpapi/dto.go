package httpapi

import (
	"time"

	"arrange-matches-bwd/internal/domain"
	"arrange-matches-bwd/internal/finance"
	"arrange-matches-bwd/internal/session"
)

type playerDTO struct {
	ID         int64         `json:"id"`
	Name       string        `json:"name"`
	Gender     domain.Gender `json:"gender"`
	Level      domain.Level  `json:"level"`
	IsRegular  bool          `json:"is_regular"`
	MonthlyFee int64         `json:"monthly_fee"`
	Note       string        `json:"note"`
}

func toPlayerDTO(p domain.Player) playerDTO {
	return playerDTO{ID: p.ID, Name: p.Name, Gender: p.Gender, Level: p.Level, IsRegular: p.IsRegular, MonthlyFee: p.MonthlyFee, Note: p.Note}
}

func (d playerDTO) toDomain() domain.Player {
	return domain.Player{ID: d.ID, Name: d.Name, Gender: d.Gender, Level: d.Level, IsRegular: d.IsRegular, MonthlyFee: d.MonthlyFee, Note: d.Note}
}

type settingsDTO struct {
	FeeMale            int64 `json:"fee_male"`
	FeeFemale          int64 `json:"fee_female"`
	CourtFee           int64 `json:"court_fee"`
	GenderGapThreshold int   `json:"gender_gap_threshold"`
	BalanceThreshold   int   `json:"balance_threshold"`
	RepeatLookback     int   `json:"repeat_lookback"`
}

func toSettingsDTO(s domain.Settings) settingsDTO {
	return settingsDTO{
		FeeMale: s.FeeMale, FeeFemale: s.FeeFemale, CourtFee: s.CourtFee,
		GenderGapThreshold: s.Rules.GenderGapThreshold, BalanceThreshold: s.Rules.BalanceThreshold, RepeatLookback: s.Rules.RepeatLookback,
	}
}

func (d settingsDTO) toDomain() domain.Settings {
	return domain.Settings{
		FeeMale: d.FeeMale, FeeFemale: d.FeeFemale, CourtFee: d.CourtFee,
		Rules: domain.MatchRules{GenderGapThreshold: d.GenderGapThreshold, BalanceThreshold: d.BalanceThreshold, RepeatLookback: d.RepeatLookback},
	}
}

type sessionDTO struct {
	ID         int64                `json:"id"`
	Date       string               `json:"date"`
	StartTime  *time.Time           `json:"start_time"`
	EndTime    *time.Time           `json:"end_time"`
	Status     domain.SessionStatus `json:"status"`
	CourtCount int                  `json:"court_count"`
	FeeMale    int64                `json:"fee_male"`
	FeeFemale  int64                `json:"fee_female"`
	CourtFee   int64                `json:"court_fee"`
}

func toSessionDTO(s domain.Session) sessionDTO {
	return sessionDTO{ID: s.ID, Date: s.Date, StartTime: s.StartTime, EndTime: s.EndTime, Status: s.Status,
		CourtCount: s.CourtCount, FeeMale: s.FeeMale, FeeFemale: s.FeeFemale, CourtFee: s.CourtFee}
}

type matchDTO struct {
	ID        int64              `json:"id"`
	Status    domain.MatchStatus `json:"status"`
	StartedAt *time.Time         `json:"started_at"`
	TeamA     []int64            `json:"team_a"`
	TeamB     []int64            `json:"team_b"`
	Diff      int                `json:"diff"`
	Balanced  bool               `json:"balanced"`
}

type courtDTO struct {
	ID     int64              `json:"id"`
	Index  int                `json:"index"`
	Status domain.CourtStatus `json:"status"`
	Match  *matchDTO          `json:"match"`
}

type sessionPlayerDTO struct {
	ID              int64               `json:"id"`
	PlayerID        int64               `json:"player_id"`
	Name            string              `json:"name"`
	Gender          domain.Gender       `json:"gender"`
	Level           domain.Level        `json:"level"`
	IsRegular       bool                `json:"is_regular"`
	Status          domain.PlayerStatus `json:"status"`
	MatchesPlayed   int                 `json:"matches_played"`
	ConsecutiveRest int                 `json:"consecutive_rest"`
	Fee             int64               `json:"fee"`
	FeeOverride     *int64              `json:"fee_override"`
	IsPaid          bool                `json:"is_paid"`
	Held            bool                `json:"held"`
	QueuePosition   int                 `json:"queue_position"`
}

type feesDTO struct {
	Expected    int64 `json:"expected"`
	Collected   int64 `json:"collected"`
	Outstanding int64 `json:"outstanding"`
}

func toFeesDTO(f domain.SessionFees) feesDTO {
	return feesDTO{Expected: f.Expected, Collected: f.Collected, Outstanding: f.Outstanding()}
}

type stateDTO struct {
	Session sessionDTO         `json:"session"`
	Courts  []courtDTO         `json:"courts"`
	Players []sessionPlayerDTO `json:"players"`
	Fees    feesDTO            `json:"fees"`
}

func toStateDTO(s *session.State) *stateDTO {
	if s == nil {
		return nil
	}
	out := &stateDTO{Session: toSessionDTO(s.Session), Fees: toFeesDTO(s.Fees), Courts: []courtDTO{}, Players: []sessionPlayerDTO{}}
	for _, c := range s.Courts {
		court := courtDTO{ID: c.ID, Index: c.Index, Status: c.Status}
		if m := c.Match; m != nil {
			dto := &matchDTO{ID: m.ID, Status: m.Status, StartedAt: m.StartedAt, Diff: m.Diff, Balanced: m.Balanced, TeamA: []int64{}, TeamB: []int64{}}
			for _, mp := range m.Players {
				if mp.Team == domain.TeamA {
					dto.TeamA = append(dto.TeamA, mp.SessionPlayerID)
				} else {
					dto.TeamB = append(dto.TeamB, mp.SessionPlayerID)
				}
			}
			court.Match = dto
		}
		out.Courts = append(out.Courts, court)
	}
	for _, p := range s.Players {
		out.Players = append(out.Players, sessionPlayerDTO{
			ID: p.ID, PlayerID: p.PlayerID, Name: p.Name, Gender: p.Gender, Level: p.Level, IsRegular: p.IsRegular,
			Status: p.Status, MatchesPlayed: p.MatchesPlayed, ConsecutiveRest: p.ConsecutiveRest,
			Fee: p.Fee, FeeOverride: p.FeeOverride, IsPaid: p.IsPaid, Held: p.Held, QueuePosition: p.QueuePosition,
		})
	}
	return out
}

type transactionDTO struct {
	ID         int64                  `json:"id"`
	SessionID  *int64                 `json:"session_id"`
	PlayerID   *int64                 `json:"player_id"`
	PlayerName string                 `json:"player_name"`
	Type       domain.TransactionType `json:"type"`
	Category   domain.Category        `json:"category"`
	Amount     int64                  `json:"amount"`
	Note       string                 `json:"note"`
	Date       string                 `json:"date"`
	Auto       bool                   `json:"auto"`
}

func toTransactionDTO(v finance.TransactionView) transactionDTO {
	return transactionDTO{ID: v.ID, SessionID: v.SessionID, PlayerID: v.PlayerID, PlayerName: v.PlayerName,
		Type: v.Type, Category: v.Category, Amount: v.Amount, Note: v.Note, Date: v.Date, Auto: v.Auto}
}

func toTransactionDTOs(views []finance.TransactionView) []transactionDTO {
	out := make([]transactionDTO, 0, len(views))
	for _, v := range views {
		out = append(out, toTransactionDTO(v))
	}
	return out
}

func (d transactionDTO) toDomain() domain.Transaction {
	return domain.Transaction{ID: d.ID, SessionID: d.SessionID, PlayerID: d.PlayerID,
		Type: d.Type, Category: d.Category, Amount: d.Amount, Note: d.Note, Date: d.Date}
}

type totalsDTO struct {
	Income  int64 `json:"income"`
	Expense int64 `json:"expense"`
	Profit  int64 `json:"profit"`
}

func toTotalsDTO(t finance.Totals) totalsDTO {
	return totalsDTO{Income: t.Income, Expense: t.Expense, Profit: t.Profit()}
}

type monthSummaryDTO struct {
	Month string `json:"month"`
	totalsDTO
	SessionCount int `json:"session_count"`
}

type summaryDTO struct {
	totalsDTO
	Months []monthSummaryDTO `json:"months"`
}

func toSummaryDTO(s finance.Summary) summaryDTO {
	out := summaryDTO{totalsDTO: toTotalsDTO(s.Totals), Months: []monthSummaryDTO{}}
	for _, m := range s.Months {
		out.Months = append(out.Months, monthSummaryDTO{Month: m.Month, totalsDTO: toTotalsDTO(m.Totals), SessionCount: m.SessionCount})
	}
	return out
}

type categoryTotalDTO struct {
	Category domain.Category        `json:"category"`
	Type     domain.TransactionType `json:"type"`
	Amount   int64                  `json:"amount"`
}

type regularPaymentDTO struct {
	PlayerID int64  `json:"player_id"`
	Name     string `json:"name"`
	Amount   int64  `json:"amount"`
}

type sessionRowDTO struct {
	Session     sessionDTO `json:"session"`
	PlayerCount int        `json:"player_count"`
	Male        int        `json:"male"`
	Female      int        `json:"female"`
	Casual      int        `json:"casual"`
	Regular     int        `json:"regular"`
	Fees        feesDTO    `json:"fees"`
	FeeIncome   int64      `json:"fee_income"`
	OtherIncome int64      `json:"other_income"`
	totalsDTO
}

type monthReportDTO struct {
	Month string `json:"month"`
	totalsDTO
	SessionCount    int                 `json:"session_count"`
	AvgIncome       int64               `json:"avg_income"`
	AvgProfit       int64               `json:"avg_profit"`
	Categories      []categoryTotalDTO  `json:"categories"`
	RegularIncome   int64               `json:"regular_income"`
	RegularPayments []regularPaymentDTO `json:"regular_payments"`
	CasualIncome    int64               `json:"casual_income"`
	OtherIncome     int64               `json:"other_income"`
	Sessions        []sessionRowDTO     `json:"sessions"`
	Unlinked        []transactionDTO    `json:"unlinked"`
	UnlinkedTotals  totalsDTO           `json:"unlinked_totals"`
}

func toMonthReportDTO(r finance.MonthReport) monthReportDTO {
	out := monthReportDTO{
		Month: r.Month, totalsDTO: toTotalsDTO(r.Totals), SessionCount: r.SessionCount,
		AvgIncome: r.AvgIncome, AvgProfit: r.AvgProfit,
		RegularIncome: r.RegularIncome, CasualIncome: r.CasualIncome, OtherIncome: r.OtherIncome,
		Categories: []categoryTotalDTO{}, RegularPayments: []regularPaymentDTO{}, Sessions: []sessionRowDTO{},
		Unlinked: toTransactionDTOs(r.Unlinked), UnlinkedTotals: toTotalsDTO(r.UnlinkedTotals),
	}
	for _, c := range r.Categories {
		out.Categories = append(out.Categories, categoryTotalDTO{Category: c.Category, Type: c.Type, Amount: c.Amount})
	}
	for _, p := range r.RegularPayments {
		out.RegularPayments = append(out.RegularPayments, regularPaymentDTO{PlayerID: p.PlayerID, Name: p.Name, Amount: p.Amount})
	}
	for _, s := range r.Sessions {
		out.Sessions = append(out.Sessions, sessionRowDTO{
			Session: toSessionDTO(s.Session), PlayerCount: s.PlayerCount, Male: s.Male, Female: s.Female,
			Casual: s.Casual, Regular: s.Regular, Fees: toFeesDTO(s.Fees),
			FeeIncome: s.FeeIncome, OtherIncome: s.OtherIncome, totalsDTO: toTotalsDTO(s.Totals()),
		})
	}
	return out
}

type sessionReportDTO struct {
	*stateDTO
	Transactions []transactionDTO `json:"transactions"`
}
