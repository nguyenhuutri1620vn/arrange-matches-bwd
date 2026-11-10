package httpapi

import (
	"encoding/json"
	"net/http"

	"arrange-matches-bwd/internal/domain"
	"arrange-matches-bwd/internal/session"
)

func (s *server) getSettings(r *http.Request) (any, error) {
	settings, err := s.sessions.Settings(r.Context())
	return toSettingsDTO(settings), err
}

func (s *server) putSettings(r *http.Request) (any, error) {
	var req settingsDTO
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	if err := s.sessions.UpdateSettings(r.Context(), req.toDomain()); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *server) listPlayers(r *http.Request) (any, error) {
	players, err := s.sessions.ListPlayers(r.Context())
	if err != nil {
		return nil, err
	}
	out := make([]playerDTO, 0, len(players))
	for _, p := range players {
		out = append(out, toPlayerDTO(p))
	}
	return out, nil
}

func (s *server) createPlayer(r *http.Request) (any, error) {
	var req playerDTO
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	req.ID = 0
	p, err := s.sessions.CreatePlayer(r.Context(), req.toDomain())
	return toPlayerDTO(p), err
}

func (s *server) updatePlayer(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var req playerDTO
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	req.ID = id
	p, err := s.sessions.UpdatePlayer(r.Context(), req.toDomain())
	return toPlayerDTO(p), err
}

func (s *server) deletePlayer(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	return map[string]bool{"deleted": true}, s.sessions.DeletePlayer(r.Context(), id)
}

func (s *server) currentSession(r *http.Request) (any, error) {
	state, err := s.sessions.Current(r.Context())
	return toStateDTO(state), err
}

func (s *server) createSession(r *http.Request) (any, error) {
	var req struct {
		CourtCount int    `json:"court_count"`
		FeeMale    *int64 `json:"fee_male"`
		FeeFemale  *int64 `json:"fee_female"`
		CourtFee   *int64 `json:"court_fee"`
	}
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	state, err := s.sessions.Create(r.Context(), session.CreateParams{
		CourtCount: req.CourtCount, FeeMale: req.FeeMale, FeeFemale: req.FeeFemale, CourtFee: req.CourtFee,
	})
	return toStateDTO(state), err
}

// stateAction adapts the usecases that take one id and return the state.
func stateAction(r *http.Request, action func(id int64) (*session.State, error)) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	state, err := action(id)
	return toStateDTO(state), err
}

func (s *server) getSession(r *http.Request) (any, error) {
	return stateAction(r, func(id int64) (*session.State, error) { return s.sessions.Get(r.Context(), id) })
}

func (s *server) endSession(r *http.Request) (any, error) {
	return stateAction(r, func(id int64) (*session.State, error) { return s.sessions.End(r.Context(), id) })
}

func (s *server) setCourtCount(r *http.Request) (any, error) {
	var req struct {
		CourtCount int `json:"court_count"`
	}
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	return stateAction(r, func(id int64) (*session.State, error) {
		return s.sessions.SetCourtCount(r.Context(), id, req.CourtCount)
	})
}

func (s *server) checkIn(r *http.Request) (any, error) {
	var req struct {
		PlayerID int64 `json:"player_id"`
	}
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	return stateAction(r, func(id int64) (*session.State, error) {
		return s.sessions.CheckIn(r.Context(), id, req.PlayerID)
	})
}

func (s *server) removeSessionPlayer(r *http.Request) (any, error) {
	return stateAction(r, func(id int64) (*session.State, error) { return s.sessions.RemovePlayer(r.Context(), id) })
}

func (s *server) setPlayerStatus(r *http.Request) (any, error) {
	var req struct {
		Status domain.PlayerStatus `json:"status"`
	}
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	return stateAction(r, func(id int64) (*session.State, error) {
		return s.sessions.SetPlayerStatus(r.Context(), id, req.Status)
	})
}

func (s *server) setPayment(r *http.Request) (any, error) {
	// fee_override distinguishes "absent" (leave as is) from null (clear).
	var req struct {
		IsPaid      *bool           `json:"is_paid"`
		FeeOverride json.RawMessage `json:"fee_override"`
	}
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	update := session.PaymentUpdate{IsPaid: req.IsPaid}
	if len(req.FeeOverride) > 0 {
		update.SetFeeOverride = true
		if err := json.Unmarshal(req.FeeOverride, &update.FeeOverride); err != nil {
			return nil, domain.ErrInvalidAmount
		}
	}
	return stateAction(r, func(id int64) (*session.State, error) {
		return s.sessions.SetPayment(r.Context(), id, update)
	})
}

func (s *server) propose(r *http.Request) (any, error) {
	return stateAction(r, func(id int64) (*session.State, error) { return s.sessions.Propose(r.Context(), id) })
}

func (s *server) swap(r *http.Request) (any, error) {
	var req struct {
		OutID int64 `json:"out_id"`
		InID  int64 `json:"in_id"`
	}
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	return stateAction(r, func(id int64) (*session.State, error) {
		return s.sessions.SwapDraftPlayer(r.Context(), id, req.OutID, req.InID)
	})
}

func (s *server) confirm(r *http.Request) (any, error) {
	var req struct {
		Force bool `json:"force"`
	}
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	return stateAction(r, func(id int64) (*session.State, error) {
		return s.sessions.Confirm(r.Context(), id, req.Force)
	})
}

func (s *server) dismiss(r *http.Request) (any, error) {
	return stateAction(r, func(id int64) (*session.State, error) { return s.sessions.DismissDraft(r.Context(), id) })
}

func (s *server) finish(r *http.Request) (any, error) {
	return stateAction(r, func(id int64) (*session.State, error) { return s.sessions.Finish(r.Context(), id) })
}

func (s *server) listTransactions(r *http.Request) (any, error) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = domain.VNDate(s.now())[:7]
	}
	views, err := s.finance.List(r.Context(), month)
	if err != nil {
		return nil, err
	}
	return toTransactionDTOs(views), nil
}

func (s *server) createTransaction(r *http.Request) (any, error) {
	var req transactionDTO
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	req.ID = 0
	view, err := s.finance.Create(r.Context(), req.toDomain())
	return toTransactionDTO(view), err
}

func (s *server) updateTransaction(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	var req transactionDTO
	if err := decode(r, &req); err != nil {
		return nil, err
	}
	req.ID = id
	view, err := s.finance.Update(r.Context(), req.toDomain())
	return toTransactionDTO(view), err
}

func (s *server) deleteTransaction(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	return map[string]bool{"deleted": true}, s.finance.Delete(r.Context(), id)
}

func (s *server) reportSummary(r *http.Request) (any, error) {
	summary, err := s.finance.Summary(r.Context())
	return toSummaryDTO(summary), err
}

func (s *server) reportMonth(r *http.Request) (any, error) {
	report, err := s.finance.Month(r.Context(), r.PathValue("month"))
	return toMonthReportDTO(report), err
}

func (s *server) reportSession(r *http.Request) (any, error) {
	id, err := pathID(r)
	if err != nil {
		return nil, err
	}
	state, err := s.sessions.Get(r.Context(), id)
	if err != nil {
		return nil, err
	}
	txs, err := s.finance.SessionTransactions(r.Context(), id)
	if err != nil {
		return nil, err
	}
	return sessionReportDTO{stateDTO: toStateDTO(state), Transactions: toTransactionDTOs(txs)}, nil
}
