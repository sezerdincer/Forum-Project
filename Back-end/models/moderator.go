package models

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

type Post struct {
	ID            int
	UserID        int
	Title         string
	Content       string
	Image         string
	CategoryID    int
	CreatedAt     time.Time
	TotalLikes    int
	TotalDislikes int
}

type PostWithFeedback struct {
	Post
	Feedback sql.NullString
}

type ModeratorPanelData struct {
	Posts []PostWithFeedback
}

func HandleModeratorPanel(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "./Back-end/database/forum.db")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	query := `
        SELECT p.id, p.user_id, p.title, p.content, p.image, p.category_id, p.created_at, p.total_likes, p.total_dislikes, IFNULL(f.feedback, '')
        FROM posts p
        LEFT JOIN (
            SELECT r.post_id, fb.feedback
            FROM reports r
            LEFT JOIN feedback fb ON r.id = fb.report_id
        ) f ON p.id = f.post_id
    `

	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var posts []PostWithFeedback
	for rows.Next() {
		var post PostWithFeedback
		if err := rows.Scan(&post.ID, &post.UserID, &post.Title, &post.Content, &post.Image, &post.CategoryID, &post.CreatedAt, &post.TotalLikes, &post.TotalDislikes, &post.Feedback); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		posts = append(posts, post)
	}

	tmpl, ok := tmplCache["moderator_panel"]
	if !ok {
		http.Error(w, "Could not load moderator panel template", http.StatusInternalServerError)
		return
	}

	data := ModeratorPanelData{
		Posts: posts,
	}

	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func HandleModeratorDeletePost(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		postID := r.FormValue("postId")

		db, err := sql.Open("sqlite3", "./Back-end/database/forum.db")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer db.Close()

		_, err = db.Exec("DELETE FROM posts WHERE id = ?", postID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/moderatorPanel", http.StatusSeeOther)
	}
}

func HandleReportPost(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("user_id")
	if err != nil {
		http.Error(w, "User ID not provided", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseInt(cookie.Value, 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodPost {
		postID := r.FormValue("postId")
		reason := r.FormValue("reason")
		moderatorID := userID // Assume this function retrieves the logged-in moderator ID

		db, err := sql.Open("sqlite3", "./Back-end/database/forum.db")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer db.Close()

		_, err = db.Exec("INSERT INTO reports (post_id, moderator_id, reason) VALUES (?, ?, ?)", postID, moderatorID, reason)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/moderatorPanel", http.StatusSeeOther)
	}
}
