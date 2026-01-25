package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/c2FmZQ/storage"
)

func TestPaginationAndSorting(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pagination_sort_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	s := storage.New(tempDir, nil)
	gStore := NewGameStore(tempDir, s)
	tStore := NewTeamStore(tempDir, s)
	us := NewUserIndexStore(tempDir, s, nil)
	reg := NewRegistry(gStore, tStore, us, true)

	_, _, handler := NewServerHandler(Options{
		GameStore:   gStore,
		TeamStore:   tStore,
		Storage:     s,
		Registry:    reg,
		UseMockAuth: true,
	})

	userId := "user@example.com"

	// Setup Games
	// Game A: Date 2025-01-01, Event "Opening Day", Location "Stadium A"
	// Game B: Date 2025-01-02, Event "Playoff", Location "Stadium B"
	// Game C: Date 2025-01-03, Event "Finals", Location "Stadium A"
	gamesData := []struct {
		ID       string
		Date     string
		Event    string
		Location string
	}{
		{"11111111-0000-0000-0000-000000000001", "2025-01-01", "Opening Day", "Stadium A"},
		{"11111111-0000-0000-0000-000000000002", "2025-01-02", "Playoff", "Stadium B"},
		{"11111111-0000-0000-0000-000000000003", "2025-01-03", "Finals", "Stadium A"},
	}

	for _, d := range gamesData {
		game := Game{
			ID:            d.ID,
			SchemaVersion: SchemaVersionV3,
			OwnerID:       userId,
			Date:          d.Date,
			Event:         d.Event,
			Location:      d.Location,
		}
		gStore.SaveGame(&game)
		reg.UpdateGame(game)
	}

	// Setup Teams
	// Team A: Name "Yankees"
	// Team B: Name "Red Sox"
	// Team C: Name "Mets"
	teamsData := []struct {
		ID   string
		Name string
	}{
		{"22222222-0000-0000-0000-000000000001", "Yankees"},
		{"22222222-0000-0000-0000-000000000002", "Red Sox"},
		{"22222222-0000-0000-0000-000000000003", "Mets"},
	}

	for _, d := range teamsData {
		team := Team{
			ID:            d.ID,
			SchemaVersion: SchemaVersionV3,
			OwnerID:       userId,
			Name:          d.Name,
		}
		tStore.SaveTeam(&team)
		reg.UpdateTeam(team)
	}

	makeRequest := func(url string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", url, nil)
		req.AddCookie(&http.Cookie{Name: "mock_auth_user", Value: userId})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}

	// --- Pagination Tests ---
	t.Run("Pagination", func(t *testing.T) {
		w := makeRequest("/api/list-games?limit=2&offset=0")
		var resp struct {
			Data []GameSummary `json:"data"`
			Meta struct {
				Total int `json:"total"`
			} `json:"meta"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Meta.Total != 3 {
			t.Errorf("Expected total 3, got %d", resp.Meta.Total)
		}
		if len(resp.Data) != 2 {
			t.Errorf("Expected 2 items, got %d", len(resp.Data))
		}
	})

	// --- Sorting Games Tests ---
	t.Run("SortGames_Date_Desc", func(t *testing.T) {
		// Default is Date Desc
		w := makeRequest("/api/list-games")
		var resp struct {
			Data []GameSummary `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Data[0].Event != "Finals" { // 2025-01-03
			t.Errorf("Expected Finals first (latest date), got %s", resp.Data[0].Event)
		}
	})

	t.Run("SortGames_Date_Asc", func(t *testing.T) {
		w := makeRequest("/api/list-games?sortBy=date&order=asc")
		var resp struct {
			Data []GameSummary `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		if resp.Data[0].Event != "Opening Day" { // 2025-01-01
			t.Errorf("Expected Opening Day first (earliest date), got %s", resp.Data[0].Event)
		}
	})

	t.Run("SortGames_Event", func(t *testing.T) {
		w := makeRequest("/api/list-games?sortBy=event&order=asc")
		var resp struct {
			Data []GameSummary `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		// Finals, Opening Day, Playoff
		if resp.Data[0].Event != "Finals" {
			t.Errorf("Expected Finals first, got %s", resp.Data[0].Event)
		}
		if resp.Data[2].Event != "Playoff" {
			t.Errorf("Expected Playoff last, got %s", resp.Data[2].Event)
		}
	})

	// --- Sorting Teams Tests ---
	t.Run("SortTeams_Name_Asc", func(t *testing.T) {
		// Default is Name Asc
		w := makeRequest("/api/list-teams")
		var resp struct {
			Data []Team `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		// Mets, Red Sox, Yankees
		if resp.Data[0].Name != "Mets" {
			t.Errorf("Expected Mets first, got %s", resp.Data[0].Name)
		}
		if resp.Data[2].Name != "Yankees" {
			t.Errorf("Expected Yankees last, got %s", resp.Data[2].Name)
		}
	})

	t.Run("SortTeams_Name_Desc", func(t *testing.T) {
		w := makeRequest("/api/list-teams?sortBy=name&order=desc")
		var resp struct {
			Data []Team `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		// Yankees, Red Sox, Mets
		if resp.Data[0].Name != "Yankees" {
			t.Errorf("Expected Yankees first, got %s", resp.Data[0].Name)
		}
	})

	// --- Filtering Tests ---
	t.Run("FilterGames", func(t *testing.T) {
		w := makeRequest("/api/list-games?q=%22Stadium+A%22")
		var resp struct {
			Data []GameSummary `json:"data"`
			Meta struct {
				Total int `json:"total"`
			} `json:"meta"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		// Should match Stadium A (2 games)
		if resp.Meta.Total != 2 {
			t.Errorf("Expected 2 games filtering by 'Stadium A', got %d", resp.Meta.Total)
		}
	})

	t.Run("FilterTeams", func(t *testing.T) {
		w := makeRequest("/api/list-teams?q=Sox")
		var resp struct {
			Data []Team `json:"data"`
			Meta struct {
				Total int `json:"total"`
			} `json:"meta"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		// Should match Red Sox
		if resp.Meta.Total != 1 {
			t.Errorf("Expected 1 team filtering by 'Sox', got %d", resp.Meta.Total)
		}
		if len(resp.Data) > 0 && resp.Data[0].Name != "Red Sox" {
			t.Errorf("Expected Red Sox, got %s", resp.Data[0].Name)
		}
	})
}
