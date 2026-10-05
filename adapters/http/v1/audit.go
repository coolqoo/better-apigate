package v1

import (
	"net/http"

	"github.com/coolqoo/better-apigate/domain/portal"
)

func (s *Server) auditTrail(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPagination(w, r)
	if !ok {
		return
	}
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM admin_audit").Scan(&page.Total); err != nil {
		failure(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT a.id,a.actor_id,COALESCE(u.email,''),a.action,a.target_id,a.reason,a.created_at
		FROM admin_audit a LEFT JOIN users u ON u.id=a.actor_id ORDER BY a.created_at DESC,a.id DESC LIMIT ? OFFSET ?`, page.Limit(), page.Offset())
	if err != nil {
		failure(w, err)
		return
	}
	defer rows.Close()
	entries := []portal.AuditEntry{}
	for rows.Next() {
		var entry portal.AuditEntry
		if err := rows.Scan(&entry.ID, &entry.ActorID, &entry.ActorEmail, &entry.Action, &entry.TargetID, &entry.Reason, &entry.CreatedAt); err != nil {
			failure(w, err)
			return
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		failure(w, err)
		return
	}
	collectionPage(w, "audit-entry", entries, page)
}
