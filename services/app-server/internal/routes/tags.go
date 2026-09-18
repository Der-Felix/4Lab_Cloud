package routes

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/4labscloud/4labscloud/services/app-server/internal/audit"
	"github.com/4labscloud/4labscloud/services/app-server/internal/auth"
	"github.com/4labscloud/4labscloud/services/app-server/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TagItem repraesentiert ein Tag/Album.
type TagItem struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Color     *string   `json:"color,omitempty"`
	FileCount int       `json:"file_count,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateTagRequest definiert die Eingabedaten zum Erstellen oder Zuweisen eines Tags.
type CreateTagRequest struct {
	Name  string  `json:"name" binding:"required,min=1,max=100"`
	Color *string `json:"color"`
}

// AssignTagRequest definiert die Parameter zum Verknuepfen eines Tags mit einer Datei.
type AssignTagRequest struct {
	TagID *uuid.UUID `json:"tag_id"`
	Name  *string    `json:"name"`
	Color *string    `json:"color"`
}

// TagsHandler verwaltet Tags und Alben mit RLS.
type TagsHandler struct {
	dbPool      *pgxpool.Pool
	auditLogger *audit.Logger
}

// NewTagsHandler initialisiert den TagsHandler.
func NewTagsHandler(dbPool *pgxpool.Pool, auditLogger *audit.Logger) *TagsHandler {
	return &TagsHandler{
		dbPool:      dbPool,
		auditLogger: auditLogger,
	}
}

// ListTags liefert alle Tags des authentifizierten Benutzers inkl. Datei-Anzahl.
func (h *TagsHandler) ListTags(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	var tags []TagItem
	err := db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		rows, err := tx.Query(c.Request.Context(),
			`SELECT t.id, t.name, t.color, t.created_at,
			        (SELECT count(*) FROM file_tags ft WHERE ft.tag_id = t.id) as file_count
			 FROM tags t
			 WHERE t.user_id = $1
			 ORDER BY t.name ASC`,
			userID,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var tag TagItem
			if err := rows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt, &tag.FileCount); err != nil {
				return err
			}
			tags = append(tags, tag)
		}
		return rows.Err()
	})

	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim abfragen der tags", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tags konnten nicht geladen werden"})
		return
	}

	if tags == nil {
		tags = []TagItem{}
	}

	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// CreateTag erstellt einen neuen Tag fuer den Benutzer.
func (h *TagsHandler) CreateTag(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	var req CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Anfrage: " + err.Error()})
		return
	}

	cleanName := strings.TrimSpace(req.Name)
	if cleanName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tag-Name darf nicht leer sein"})
		return
	}

	var tag TagItem
	err := db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		return tx.QueryRow(c.Request.Context(),
			`INSERT INTO tags (user_id, name, color)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (user_id, name) DO UPDATE SET color = coalesce(EXCLUDED.color, tags.color)
			 RETURNING id, name, color, created_at`,
			userID, cleanName, req.Color,
		).Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt)
	})

	if err != nil {
		slog.ErrorContext(c.Request.Context(), "fehler beim erstellen des tags", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tag konnte nicht erstellt werden"})
		return
	}

	h.logAudit(c.Request.Context(), &userID, "tag_create", &tag.ID, c.ClientIP(), "ok")

	c.JSON(http.StatusCreated, tag)
}

// AssignTag weist einer Datei ein Tag zu (erstellt das Tag falls noetig).
func (h *TagsHandler) AssignTag(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	fileIDStr := c.Param("id")
	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Datei-ID"})
		return
	}

	var req AssignTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Anfrage: " + err.Error()})
		return
	}

	var targetTagID uuid.UUID
	err = db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		// 1. Pruefen ob Datei dem User gehoert
		var count int
		if err := tx.QueryRow(c.Request.Context(), "SELECT count(*) FROM files WHERE id = $1", fileID).Scan(&count); err != nil || count == 0 {
			return pgx.ErrNoRows
		}

		// 2. Tag-ID ermitteln oder Tag erstellen
		if req.TagID != nil {
			targetTagID = *req.TagID
		} else if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
			cleanName := strings.TrimSpace(*req.Name)
			err := tx.QueryRow(c.Request.Context(),
				`INSERT INTO tags (user_id, name, color)
				 VALUES ($1, $2, $3)
				 ON CONFLICT (user_id, name) DO UPDATE SET color = coalesce(EXCLUDED.color, tags.color)
				 RETURNING id`,
				userID, cleanName, req.Color,
			).Scan(&targetTagID)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("weder tag_id noch name angegeben")
		}

		// 3. In file_tags eintragen
		_, err := tx.Exec(c.Request.Context(),
			`INSERT INTO file_tags (file_id, tag_id)
			 VALUES ($1, $2)
			 ON CONFLICT (file_id, tag_id) DO NOTHING`,
			fileID, targetTagID,
		)
		return err
	})

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Datei nicht gefunden"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tag konnte nicht zugewiesen werden"})
		return
	}

	h.logAudit(c.Request.Context(), &userID, "tag_assign", &fileID, c.ClientIP(), "ok")

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"file_id": fileID,
		"tag_id":  targetTagID,
	})
}

// RemoveTag entfernt die Tag-Zuweisung von einer Datei.
func (h *TagsHandler) RemoveTag(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	fileIDStr := c.Param("id")
	fileID, err := uuid.Parse(fileIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Datei-ID"})
		return
	}

	tagIDStr := c.Param("tag_id")
	tagID, err := uuid.Parse(tagIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Tag-ID"})
		return
	}

	err = db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		cmd, err := tx.Exec(c.Request.Context(),
			"DELETE FROM file_tags WHERE file_id = $1 AND tag_id = $2",
			fileID, tagID,
		)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() == 0 {
			return pgx.ErrNoRows
		}
		return nil
	})

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Verknuepfung nicht gefunden"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Tag konnte nicht entfernt werden"})
		return
	}

	h.logAudit(c.Request.Context(), &userID, "tag_remove", &fileID, c.ClientIP(), "ok")

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ListFilesByTag liefert alle Dateien, die einem bestimmten Tag zugeordnet sind (Album-Ansicht).
func (h *TagsHandler) ListFilesByTag(c *gin.Context) {
	userID := auth.MustGetUserID(c)
	if userID == uuid.Nil {
		return
	}

	tagIDStr := c.Param("id")
	tagID, err := uuid.Parse(tagIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungueltige Tag-ID"})
		return
	}

	var (
		tagName  string
		tagColor *string
		files    []FileItem
	)

	err = db.WithUserRLS(c.Request.Context(), h.dbPool, userID, func(tx pgx.Tx) error {
		// Tag pruefen
		if err := tx.QueryRow(c.Request.Context(), "SELECT name, color FROM tags WHERE id = $1", tagID).Scan(&tagName, &tagColor); err != nil {
			return err
		}

		rows, err := tx.Query(c.Request.Context(),
			`SELECT f.id, f.filename, f.size_bytes, coalesce(f.mime_type, 'application/octet-stream'),
			        f.thumbnail_path, f.width, f.height, f.taken_at, f.created_at
			 FROM files f
			 JOIN file_tags ft ON f.id = ft.file_id
			 WHERE ft.tag_id = $1
			 ORDER BY coalesce(f.taken_at, f.created_at) DESC`,
			tagID,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var f FileItem
			if err := rows.Scan(&f.ID, &f.Filename, &f.SizeBytes, &f.MimeType, &f.ThumbnailPath, &f.Width, &f.Height, &f.TakenAt, &f.CreatedAt); err != nil {
				return err
			}
			files = append(files, f)
		}
		return rows.Err()
	})

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Album nicht gefunden"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dateien konnten nicht geladen werden"})
		return
	}

	if files == nil {
		files = []FileItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"tag_id": tagID,
		"name":   tagName,
		"color":  tagColor,
		"files":  files,
	})
}

func (h *TagsHandler) logAudit(ctx context.Context, userID *uuid.UUID, action string, targetID *uuid.UUID, ip, result string) {
	if h.auditLogger != nil {
		h.auditLogger.Log(ctx, audit.Entry{
			UserID:   userID,
			Action:   action,
			TargetID: targetID,
			IP:       ip,
			Result:   result,
		})
	}
}
