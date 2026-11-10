package domain

// ErrorKind tells the transport layer how to classify a business error.
type ErrorKind int

const (
	KindInvalid ErrorKind = iota
	KindNotFound
	KindConflict
)

// Error is a business-rule error. Compare with errors.Is against the
// sentinels below; Message is the Vietnamese text shown to the host.
type Error struct {
	Kind    ErrorKind
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func newErr(kind ErrorKind, code, msg string) *Error {
	return &Error{Kind: kind, Code: code, Message: msg}
}

var (
	ErrNotFound        = newErr(KindNotFound, "not_found", "Không tìm thấy dữ liệu.")
	ErrInvalidInput    = newErr(KindInvalid, "invalid_input", "Dữ liệu không hợp lệ.")
	ErrInvalidLevel    = newErr(KindInvalid, "invalid_level", "Level không hợp lệ.")
	ErrInvalidGender   = newErr(KindInvalid, "invalid_gender", "Giới tính không hợp lệ.")
	ErrInvalidName     = newErr(KindInvalid, "invalid_name", "Tên không được để trống.")
	ErrInvalidAmount   = newErr(KindInvalid, "invalid_amount", "Số tiền không hợp lệ.")
	ErrInvalidDate     = newErr(KindInvalid, "invalid_date", "Ngày không hợp lệ.")
	ErrInvalidCategory = newErr(KindInvalid, "invalid_category", "Danh mục không hợp lệ.")

	ErrNotEnoughPlayers    = newErr(KindConflict, "not_enough_players", "Chưa đủ 4 người trong hàng chờ.")
	ErrNoValidLineup       = newErr(KindConflict, "no_valid_lineup", "Không có đội hình hợp lệ từ hàng chờ hiện tại.")
	ErrInvalidGenderLineup = newErr(KindConflict, "invalid_gender_lineup", "Đội hình không hợp lệ về giới tính.")
	ErrUnbalancedLineup    = newErr(KindConflict, "unbalanced_lineup", "Hai đội đang lệch level.")

	ErrSessionAlreadyOpen = newErr(KindConflict, "session_already_open", "Đang có một buổi chưa kết thúc.")
	ErrSessionEnded       = newErr(KindConflict, "session_ended", "Buổi đã kết thúc, chỉ sửa được phần thu chi.")
	ErrInvalidCourtCount  = newErr(KindInvalid, "invalid_court_count", "Số sân phải từ 1 đến 3.")
	ErrCourtBusy          = newErr(KindConflict, "court_busy", "Không thể giảm số sân: sân bị đóng đang đánh hoặc đang có đề xuất.")
	ErrCourtNotEmpty      = newErr(KindConflict, "court_not_empty", "Sân đang có trận hoặc đã có đề xuất.")
	ErrCourtClosed        = newErr(KindConflict, "court_closed", "Sân này không mở trong buổi.")

	ErrAlreadyCheckedIn  = newErr(KindConflict, "already_checked_in", "Người này đã có trong buổi.")
	ErrPlayerDeleted     = newErr(KindConflict, "player_deleted", "Người chơi đã bị xóa khỏi danh bạ.")
	ErrPlayerHasPlayed   = newErr(KindConflict, "player_has_played", "Không thể xóa khỏi buổi: người này đã đánh hoặc đang trong trận.")
	ErrPlayerPlaying     = newErr(KindConflict, "player_playing", "Người này đang thi đấu.")
	ErrPlayerNotWaiting  = newErr(KindConflict, "player_not_waiting", "Người này không ở trong hàng chờ.")
	ErrPlayerHeld        = newErr(KindConflict, "player_held", "Người này đang nằm trong đề xuất của sân khác.")
	ErrPlayerNotInMatch  = newErr(KindConflict, "player_not_in_match", "Người này không nằm trong trận.")
	ErrInvalidTransition = newErr(KindConflict, "invalid_transition", "Không thể chuyển trạng thái.")
	ErrMatchNotDraft     = newErr(KindConflict, "match_not_draft", "Trận này không còn là đề xuất.")
	ErrMatchNotOngoing   = newErr(KindConflict, "match_not_ongoing", "Trận này không đang diễn ra.")
	ErrTransactionIsAuto = newErr(KindConflict, "transaction_is_auto", "Khoản phí buổi được tính tự động theo cờ đã thu, không sửa trực tiếp.")
	ErrNotRegularPlayer  = newErr(KindConflict, "not_regular_player", "Phí tháng chỉ dành cho người cố định.")
)
