package api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// allowedDatasetTables is the explicit allow-list for Parquet export.
// No user input ever reaches the ClickHouse query string directly.
var allowedDatasetTables = map[string]string{
	"block_tips":               "marts.fct_blocks",
	"tx_fees":                  "marts.fct_transactions",
	"swap_markouts":            "marts.fct_swap_markouts",
	"cex_dex_spread_per_block": "marts.fct_spread_per_block",
	"execution_quality_daily":  "marts.fct_execution_quality_daily",
	"treasury_daily":           "marts.fct_treasury_daily",
}

const maxDatasetDays = 31

// registerDatasetHandlers mounts the /api/v1/datasets/parquet/{table} route directly
// on the chi router (bypassing huma) because the handler streams binary Parquet from
// ClickHouse with io.Copy and cannot be expressed as a typed huma response body.
func (s *Server) registerDatasetRoutes() {
	s.router.With(
		s.rateLimiterDatasets,
	).Get("/api/v1/datasets/parquet/{table}", s.handleParquetExport)
}

func (s *Server) handleParquetExport(w http.ResponseWriter, r *http.Request) {
	// Extract and validate {table} from the URL path.
	parts := strings.Split(r.URL.Path, "/")
	table := ""
	for i, p := range parts {
		if p == "parquet" && i+1 < len(parts) {
			table = parts[i+1]
			break
		}
	}

	chTable, ok := allowedDatasetTables[table]
	if !ok {
		http.Error(w, `{"status":404,"title":"Not Found","detail":"unknown dataset table"}`, http.StatusNotFound)
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" || toStr == "" {
		http.Error(w, `{"status":400,"title":"Bad Request","detail":"from and to (YYYY-MM-DD) are required"}`, http.StatusBadRequest)
		return
	}

	from, err := time.ParseInLocation("2006-01-02", fromStr, time.UTC)
	if err != nil {
		http.Error(w, `{"status":400,"title":"Bad Request","detail":"from must be YYYY-MM-DD"}`, http.StatusBadRequest)
		return
	}
	to, err := time.ParseInLocation("2006-01-02", toStr, time.UTC)
	if err != nil {
		http.Error(w, `{"status":400,"title":"Bad Request","detail":"to must be YYYY-MM-DD"}`, http.StatusBadRequest)
		return
	}
	if to.Before(from) {
		http.Error(w, `{"status":400,"title":"Bad Request","detail":"to must be >= from"}`, http.StatusBadRequest)
		return
	}
	if int(to.Sub(from).Hours()/24) > maxDatasetDays {
		http.Error(w, fmt.Sprintf(`{"status":400,"title":"Bad Request","detail":"window exceeds %d days"}`, maxDatasetDays), http.StatusBadRequest)
		return
	}

	// Build the ClickHouse HTTP query. The table name is from the allow-list (not user input).
	// Use toDate() filtering on a suitable date column per table.
	dateCol := datasetDateCol(table)
	query := fmt.Sprintf(
		"SELECT * FROM %s FINAL WHERE toDate(%s) BETWEEN '%s' AND '%s' FORMAT Parquet",
		chTable, dateCol, from.Format("2006-01-02"), to.Format("2006-01-02"),
	)

	chURL := s.cfg.ClickHouseHTTPAddr + "/?query=" + url.QueryEscape(query)

	// chURL is from cfg.ClickHouseHTTPAddr (env config) plus an allow-listed query; not user-controlled.
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, chURL, nil) //nolint:gosec
	if err != nil {
		http.Error(w, `{"status":500,"title":"Internal Server Error","detail":"failed to build query"}`, http.StatusInternalServerError)
		return
	}
	if s.cfg.ClickHouseROUser != "" {
		req.SetBasicAuth(s.cfg.ClickHouseROUser, s.cfg.ClickHouseROPassword)
	}

	resp, err := http.DefaultClient.Do(req) //nolint:gosec
	if err != nil {
		http.Error(w, `{"status":502,"title":"Bad Gateway","detail":"ClickHouse unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.Copy(w, resp.Body)
		return
	}

	filename := fmt.Sprintf("%s_%s_%s.parquet", table, fromStr, toStr)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, resp.Body)
}

// datasetDateCol returns the date/timestamp column to use for range filtering
// for each allowed dataset table.
func datasetDateCol(table string) string {
	switch table {
	case "block_tips":
		return "block_timestamp"
	case "tx_fees":
		return "block_timestamp"
	case "swap_markouts":
		return "block_number"
	case "cex_dex_spread_per_block":
		return "block_number"
	case "execution_quality_daily", "treasury_daily":
		return "date"
	default:
		return "date"
	}
}
