package model

import (
	"testing"
	"time"
)

func TestShare_IsExpired_ReturnsTrueForExpiredShare(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	s := &Share{ExpiresAt: &past}
	if !s.IsExpired() {
		t.Error("expected IsExpired to return true for expired share")
	}
}

func TestShare_IsExpired_ReturnsFalseForNonExpiredShare(t *testing.T) {
	future := time.Now().Add(time.Hour)
	s := &Share{ExpiresAt: &future}
	if s.IsExpired() {
		t.Error("expected IsExpired to return false for non-expired share")
	}
}

func TestShare_IsExpired_ReturnsFalseWhenExpiresAtIsNil(t *testing.T) {
	s := &Share{}
	if s.IsExpired() {
		t.Error("expected IsExpired to return false when ExpiresAt is nil")
	}
}

func TestShare_IsTextShare(t *testing.T) {
	s := &Share{ContentType: "text"}
	if !s.IsTextShare() {
		t.Error("expected IsTextShare to return true for content type 'text'")
	}
	if s.IsFileShare() {
		t.Error("expected IsFileShare to return false for content type 'text'")
	}
}

func TestShare_IsFileShare(t *testing.T) {
	s := &Share{ContentType: "file"}
	if !s.IsFileShare() {
		t.Error("expected IsFileShare to return true for content type 'file'")
	}
	if s.IsTextShare() {
		t.Error("expected IsTextShare to return false for content type 'file'")
	}
}
