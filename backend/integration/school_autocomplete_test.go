package integration_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"akademi-bimbel/internal/model"
	"github.com/stretchr/testify/require"
)

func TestSchoolAutocomplete_Integration(t *testing.T) {
	env := newTestEnv(t)
	_, err := env.pool.Exec(t.Context(), `INSERT INTO school(name,code,npsn,school_types,provinsi_id,kota_id,status)
 SELECT 'AAA School '||g,'autocomplete-'||g,'X'||lpad(g::text,7,'0'),ARRAY['SD'],'35','3578','active' FROM generate_series(1,1001)g;
 INSERT INTO school(name,code,npsn,school_types,provinsi_id,kota_id,status) VALUES
 ('Needle, Academy','needle-1','X9000001',ARRAY['SD','SMP'],'35','3578','active'),
 ('Needle, Academy','needle-2','X9000002',ARRAY['SD'],'35','3578','active'),
 ('Needle Away','needle-3','X9000003',ARRAY['SD'],'33','3374','active'),
 ('Needle Hidden','needle-4','X9000004',ARRAY['SD'],'35','3578','deactivated'),
 ('100% Bintang','needle-5','X9000005',ARRAY['SD'],'35','3578','active'),
 ('1000 Bintang','needle-6','X9000006',ARRAY['SD'],'35','3578','active')`)
	require.NoError(t, err)
	type page struct {
		Data       []model.SchoolOption `json:"data"`
		NextCursor string               `json:"next_cursor"`
	}
	get := func(path string, status int) page {
		t.Helper()
		resp, err := http.Get(env.server.URL + "/api/v1" + path)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, status, resp.StatusCode)
		var p page
		if status == 200 {
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&p))
		}
		return p
	}
	for _, tc := range []struct {
		name, query string
		count       int
	}{
		{"global", "q=needle", 3}, {"province only", "q=needle&province_id=35", 2},
		{"province and city", "q=needle&province_id=35&city_id=3578", 2}, {"type only", "q=needle&school_type=SMP", 1},
		{"combined", "q=needle&province_id=33&city_id=3374&school_type=SD", 1},
		{"filter without name", "school_type=SMP", 1},
		{"bounded province without name", "province_id=35&limit=2", 2},
		{"literal wildcard", "q=100%25", 1}, {"short unscoped query", "q=n", 0}, {"two characters", "q=ne", 0}, {"two characters with filter", "q=ne&province_id=35", 0}, {"three characters", "q=nee", 3}, {"empty unscoped", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) { p := get("/schools?"+tc.query, 200); require.Len(t, p.Data, tc.count) })
	}
	first := get("/schools?q=needle&province_id=35&city_id=3578&limit=1", 200)
	require.Len(t, first.Data, 1)
	require.NotEmpty(t, first.NextCursor)
	second := get("/schools?q=needle&province_id=35&city_id=3578&limit=1&cursor="+url.QueryEscape(first.NextCursor), 200)
	require.Len(t, second.Data, 1)
	require.NotEqual(t, first.Data[0].ID, second.Data[0].ID)
	require.Empty(t, second.NextCursor)
	get("/schools?q=needle&province_id=33&city_id=3578", 400)
	for _, query := range []string{"city_id=3578", "q=needle&city_id=3578", "city_id=3578&school_type=SD", "province_id=%20&city_id=3578"} {
		get("/schools?"+query, 400)
	}
	get("/schools?q=needle&province_id=35&city_id=missing", 400)
	get("/schools?q=needle&school_type=invalid", 400)
	get("/schools?q=needle&cursor=bad", 400)
	exact := get("/schools?npsn=X9000001", 200)
	require.Len(t, exact.Data, 1)
	require.Equal(t, "Needle, Academy", exact.Data[0].Name)
	resp, err := http.Get(env.server.URL + "/api/v1/cities")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
