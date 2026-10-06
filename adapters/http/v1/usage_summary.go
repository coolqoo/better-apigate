package v1

import (
	"net/http"

	"github.com/coolqoo/better-apigate/domain/portal"
)

// Spending is derived from the complete immutable ledger, never a history page
// or today's price multiplied by historical units.
func (s *Server) usageSummary(w http.ResponseWriter, r *http.Request) {
	userID := current(r).UserID
	var summary portal.UsageSummary
	err := s.DB.QueryRowContext(r.Context(), `WITH request_totals AS (
        SELECT COUNT(*) AS requests, COUNT(*) FILTER(WHERE status_code>=400) AS errors, COALESCE(SUM(units),0) AS units
        FROM usage_events WHERE user_id=? AND timestamp>CURRENT_TIMESTAMP-INTERVAL '30 days'
    ), spending_totals AS (
        SELECT COALESCE(-SUM(amount_micros),0) AS spending,
        COALESCE(-SUM(amount_micros) FILTER(WHERE kind='usage_charged'),0) AS usage_spending,
        COALESCE(-SUM(amount_micros) FILTER(WHERE kind='plan_purchase'),0) AS plan_spending
        FROM wallet_ledger WHERE user_id=? AND created_at>CURRENT_TIMESTAMP-INTERVAL '30 days'
        AND kind IN ('usage_charged','plan_purchase') AND amount_micros<0
    ) SELECT requests,errors,units,spending,usage_spending,plan_spending FROM request_totals CROSS JOIN spending_totals`, userID, userID).Scan(
		&summary.Requests, &summary.Errors, &summary.Units, &summary.Spending, &summary.UsageSpending, &summary.PlanSpending)
	if err != nil {
		failure(w, err)
		return
	}
	resource(w, 200, "usage-summary", userID, summary)
}

func (s *Server) meteredUsage(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.QueryContext(r.Context(), `
		SELECT e.route_id || ':' || e.metering_unit,
		       COALESCE(r.name, 'Removed route'), COALESCE(r.path_pattern, MIN(e.path)),
		       e.metering_unit, SUM(e.metered_value), COUNT(*)
		FROM usage_events e LEFT JOIN routes r ON r.id=e.route_id
		WHERE e.user_id=? AND e.timestamp>CURRENT_TIMESTAMP-INTERVAL '30 days'
		      AND e.metering_unit<>''
		GROUP BY e.route_id,e.metering_unit,r.name,r.path_pattern ORDER BY 2,4`, current(r).UserID)
	if err != nil {
		failure(w, err)
		return
	}
	defer rows.Close()
	out := []portal.MeteredUsage{}
	for rows.Next() {
		var item portal.MeteredUsage
		if err = rows.Scan(&item.ID, &item.Name, &item.Path, &item.Unit, &item.Quantity, &item.Requests); err != nil {
			failure(w, err)
			return
		}
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		failure(w, err)
		return
	}
	collection(w, "metered-usage", out)
}
