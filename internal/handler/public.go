package handler

import (
	"log/slog"
	"net/http"

	"fls/internal/service"
)

type PublicHandler struct {
	shareSvc *service.ShareService
}

func NewPublicHandler(shareSvc *service.ShareService) *PublicHandler {
	return &PublicHandler{shareSvc: shareSvc}
}

type publicShareItem struct {
	Token       string
	ContentType string
	FileName    string
	TextPreview string
}

func (h *PublicHandler) GetPublicIndex(w http.ResponseWriter, r *http.Request) {
	shares, err := h.shareSvc.ListFeaturedShares()
	if err != nil {
		slog.Error("failed to list featured shares", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	items := make([]publicShareItem, len(shares))
	for i, share := range shares {
		item := publicShareItem{
			Token:       share.Token,
			ContentType: share.ContentType,
		}
		if share.IsTextShare() {
			item.TextPreview = truncateText(share.TextContent, 200)
		} else {
			item.FileName = share.FileName
		}
		items[i] = item
	}

	RenderTemplate(w, "public-index", map[string]interface{}{
		"Authenticated": false,
		"Shares":        items,
	})
}
