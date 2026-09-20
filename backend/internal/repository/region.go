package repository

import (
	"context"

	"akademi-bimbel/internal/model"
)

// ListProvinces returns all provinces ordered alphabetically by name.
func (r *Repository) ListProvinces(ctx context.Context) ([]model.Province, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name FROM province ORDER BY name ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var provinces []model.Province
	for rows.Next() {
		var p model.Province
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, err
		}
		provinces = append(provinces, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return provinces, nil
}

func (r *Repository) ListCitiesByProvince(ctx context.Context, provinceID string) ([]model.City, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, province_id, name FROM city WHERE province_id = $1 ORDER BY name ASC`,
		provinceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cities []model.City
	for rows.Next() {
		var c model.City
		if err := rows.Scan(&c.ID, &c.ProvinceID, &c.Name); err != nil {
			return nil, err
		}
		cities = append(cities, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return cities, nil
}

// ListDistrictsByCity returns all districts belonging to the given city,
// ordered alphabetically by name. Returns an empty slice (not an error) when
// the city id does not exist.
func (r *Repository) ListDistrictsByCity(ctx context.Context, cityID string) ([]model.District, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, city_id, name FROM district WHERE city_id = $1 ORDER BY name ASC`,
		cityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var districts []model.District
	for rows.Next() {
		var d model.District
		if err := rows.Scan(&d.ID, &d.CityID, &d.Name); err != nil {
			return nil, err
		}
		districts = append(districts, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return districts, nil
}

// GetProvinceByName returns a province by its name (case-insensitive), or nil, nil when not found.
func (r *Repository) GetProvinceByName(ctx context.Context, name string) (*model.Province, error) {
	p := &model.Province{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name FROM province WHERE LOWER(name) = LOWER($1)`,
		name,
	).Scan(&p.ID, &p.Name)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// GetCityByNameInProvince returns a city by its name within a given province
// (case-insensitive), or nil, nil when not found.
func (r *Repository) GetCityByNameInProvince(ctx context.Context, name, provinceID string) (*model.City, error) {
	c := &model.City{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, province_id, name FROM city WHERE LOWER(name) = LOWER($1) AND province_id = $2`,
		name, provinceID,
	).Scan(&c.ID, &c.ProvinceID, &c.Name)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

// GetDistrictByNameInCity returns a district by its name within a given city
// (case-insensitive), or nil, nil when not found.
func (r *Repository) GetDistrictByNameInCity(ctx context.Context, name, cityID string) (*model.District, error) {
	d := &model.District{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, city_id, name FROM district WHERE LOWER(name) = LOWER($1) AND city_id = $2`,
		name, cityID,
	).Scan(&d.ID, &d.CityID, &d.Name)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return d, nil
}

// GetProvinceByID returns a province by its ID, or nil, nil when not found.
func (r *Repository) GetProvinceByID(ctx context.Context, id string) (*model.Province, error) {
	p := &model.Province{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name FROM province WHERE id = $1`,
		id,
	).Scan(&p.ID, &p.Name)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// GetCityByID returns a city by its ID, or nil, nil when not found.
func (r *Repository) GetCityByID(ctx context.Context, id string) (*model.City, error) {
	c := &model.City{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, province_id, name FROM city WHERE id = $1`,
		id,
	).Scan(&c.ID, &c.ProvinceID, &c.Name)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

// GetDistrictByID returns a district by its ID, or nil, nil when not found.
func (r *Repository) GetDistrictByID(ctx context.Context, id string) (*model.District, error) {
	d := &model.District{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, city_id, name FROM district WHERE id = $1`,
		id,
	).Scan(&d.ID, &d.CityID, &d.Name)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return d, nil
}
