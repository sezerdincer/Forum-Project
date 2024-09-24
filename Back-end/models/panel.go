package models

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

type ModeratorRequest struct {
	ID       int
	Username string
	Status   string
}

func HandleFeedbackSubmission(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		reportID := r.FormValue("reportId")
		feedback := r.FormValue("feedback")

		db, err := sql.Open("sqlite3", "./Back-end/database/forum.db")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer db.Close()

		// Insert or update feedback
		_, err = db.Exec("INSERT INTO feedback (report_id, feedback) VALUES (?, ?) ON CONFLICT(report_id) DO UPDATE SET feedback = ?", reportID, feedback, feedback)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/panel", http.StatusSeeOther)
	}
}

func HandleAdminPanel(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "./Back-end/database/forum.db")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Fetch moderator requests
	requestRows, err := db.Query("SELECT mr.id, u.username, mr.status FROM moderator_requests mr JOIN users u ON mr.user_id = u.id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer requestRows.Close()

	var requests []ModeratorRequest
	for requestRows.Next() {
		var req ModeratorRequest
		if err := requestRows.Scan(&req.ID, &req.Username, &req.Status); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		requests = append(requests, req)
	}

	// Fetch reports with post photo URLs
	reportRows, err := db.Query("SELECT r.id, p.id, p.title, u.username, r.reason, r.reported_at, p.image FROM reports r JOIN posts p ON r.post_id = p.id JOIN users u ON r.moderator_id = u.id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer reportRows.Close()

	var reports []struct {
		ID         int
		PostID     int
		PostTitle  string
		Moderator  string
		Reason     string
		ReportedAt string
		PhotoURL   string
	}

	for reportRows.Next() {
		var r struct {
			ID         int
			PostID     int
			PostTitle  string
			Moderator  string
			Reason     string
			ReportedAt string
			PhotoURL   string
		}
		if err := reportRows.Scan(&r.ID, &r.PostID, &r.PostTitle, &r.Moderator, &r.Reason, &r.ReportedAt, &r.PhotoURL); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		reports = append(reports, r)
	}

	tmpl, ok := tmplCache["panel"]
	if !ok {
		http.Error(w, "Could not load admin panel template", http.StatusInternalServerError)
		return
	}

	data := struct {
		Requests []ModeratorRequest
		Reports  []struct {
			ID         int
			PostID     int
			PostTitle  string
			Moderator  string
			Reason     string
			ReportedAt string
			PhotoURL   string
		}
	}{
		Requests: requests,
		Reports:  reports,
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func HandleApproveRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		requestID := r.FormValue("requestId")

		db, err := sql.Open("sqlite3", "./Back-end/database/forum.db")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer db.Close()

		_, err = db.Exec("UPDATE moderator_requests SET status = 'approved' WHERE id = ?", requestID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var userID int
		err = db.QueryRow("SELECT user_id FROM moderator_requests WHERE id = ?", requestID).Scan(&userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = db.Exec("UPDATE users SET is_moderator = 1 WHERE id = ?", userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/panel", http.StatusSeeOther)
	}
}

func HandleRejectRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		requestID := r.FormValue("requestId")

		db, err := sql.Open("sqlite3", "./Back-end/database/forum.db")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer db.Close()

		_, err = db.Exec("UPDATE moderator_requests SET status = 'rejected' WHERE id = ?", requestID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var userID int
		err = db.QueryRow("SELECT user_id FROM moderator_requests WHERE id = ?", requestID).Scan(&userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = db.Exec("UPDATE users SET is_moderator = 0 WHERE id = ?", userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/panel", http.StatusSeeOther)
	}
}

func HandleRevokeModerator(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		requestIDStr := r.FormValue("requestId")
		requestID, err := strconv.Atoi(requestIDStr)
		if err != nil {
			http.Error(w, "Invalid request ID", http.StatusBadRequest)
			return
		}

		log.Printf("Revoke request for request ID: %d", requestID) // Debugging log

		db, err := sql.Open("sqlite3", "./Back-end/database/forum.db")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer db.Close()

		tx, err := db.Begin()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		// Get the user ID associated with the request ID
		var userID int
		err = tx.QueryRow("SELECT user_id FROM moderator_requests WHERE id = ?", requestID).Scan(&userID)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Request ID not found", http.StatusNotFound)
			return
		}

		// Remove moderator status
		_, err = tx.Exec("UPDATE users SET is_moderator = 0 WHERE id = ?", userID)
		if err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Optionally remove admin status if needed
		_, err = tx.Exec("UPDATE users SET is_admin = 0 WHERE id = ?", userID)
		if err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Update the request status to "revoked"
		_, err = tx.Exec("UPDATE moderator_requests SET status = 'revoked' WHERE id = ?", requestID)
		if err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Commit transaction
		if err := tx.Commit(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/panel", http.StatusSeeOther)
	} else {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
	}
}

func HandleSubmitFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		reportID := r.FormValue("reportId")
		feedback := r.FormValue("feedback")

		db, err := sql.Open("sqlite3", "./Back-end/database/forum.db")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer db.Close()

		_, err = db.Exec("INSERT INTO feedback (report_id, feedback) VALUES (?, ?)", reportID, feedback)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/panel", http.StatusSeeOther)
	} else {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
	}
}
