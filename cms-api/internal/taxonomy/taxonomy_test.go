package taxonomy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListIncludesReferenceCount(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&Term{}, &Reference{}); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	category := Term{Kind: "category", Name: "技术", URL: "tech", Version: 1}
	other := Term{Kind: "category", Name: "生活", URL: "life", Version: 1}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create other category: %v", err)
	}
	for _, reference := range []Reference{
		{OwnerType: "draft", OwnerID: 1, TermID: category.ID},
		{OwnerType: "revision", OwnerID: 2, TermID: category.ID},
	} {
		if err := db.Create(&reference).Error; err != nil {
			t.Fatalf("create reference: %v", err)
		}
	}

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/categories", nil)
	NewHandler(db).list("category")(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response struct {
		Data []struct {
			ID             int64 `json:"id"`
			ReferenceCount int64 `json:"referenceCount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 2 {
		t.Fatalf("list length = %d, want 2", len(response.Data))
	}
	if response.Data[0].ID != category.ID || response.Data[0].ReferenceCount != 2 {
		t.Fatalf("first row = %+v, want category %d with 2 references", response.Data[0], category.ID)
	}
	if response.Data[1].ID != other.ID || response.Data[1].ReferenceCount != 0 {
		t.Fatalf("second row = %+v, want category %d with 0 references", response.Data[1], other.ID)
	}
}
