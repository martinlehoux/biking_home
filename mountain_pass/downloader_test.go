package mountain_pass

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMountainPasses(t *testing.T) {
	csv := "Brevet\tcode\tnom\taltitude\n" +
		"FR-01\tFR-01-1500\tCol du Colombier\t1498\n" +
		"FR-06\tFR-06-0730c | FR-06-0706\tCol de Castillon | Col de Castillon (Tunnel)\t730 | 706\n" +
		"FR-13\tFR-13-0135\tCol du Télégraphe \"Chappe\"\t149\n" +
		"FR-90\tFR-88-1171\tCol du Ballon d'Alsace\t1171\n"
	mountainPasses, err := parseMountainPasses(strings.NewReader(csv))
	require.NoError(t, err)

	expected := []MountainPass{
		{ExternalID: "centcols/FR-01-1500", CountryCode: "FR", DepartmentCode: "01", Name: "Col du Colombier", Elevation: 1498},
		{ExternalID: "centcols/FR-06-0730c", CountryCode: "FR", DepartmentCode: "06", Name: "Col de Castillon", Elevation: 730},
		{ExternalID: "centcols/FR-06-0706", CountryCode: "FR", DepartmentCode: "06", Name: "Col de Castillon (Tunnel)", Elevation: 706},
		{ExternalID: "centcols/FR-13-0135", CountryCode: "FR", DepartmentCode: "13", Name: "Col du Télégraphe \"Chappe\"", Elevation: 149},
		{ExternalID: "centcols/FR-88-1171", CountryCode: "FR", DepartmentCode: "90", Name: "Col du Ballon d'Alsace", Elevation: 1171},
	}
	assert.Equal(t, expected, mountainPasses)
}

func TestParseMountainPassesHeaderOnly(t *testing.T) {
	mountainPasses, err := parseMountainPasses(strings.NewReader("Brevet\tcode\tnom\taltitude\n"))
	require.NoError(t, err)
	assert.Empty(t, mountainPasses)
}

func TestSplitCountryDepartment(t *testing.T) {
	country, department, err := splitCountryDepartment("FR-01")
	require.NoError(t, err)
	assert.Equal(t, "FR", country)
	assert.Equal(t, "01", department)

	_, _, err = splitCountryDepartment("F-01-2")
	assert.Error(t, err)
}

func TestLoadCachedDepartment(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "debug_department_01.csv")
	err := os.WriteFile(cacheFile, []byte("Brevet\tcode\tnom\taltitude\nFR-01\tFR-01-1500\tCol du Colombier\t1498\n"), 0o644)
	require.NoError(t, err)

	mountainPasses, found, err := loadCachedDepartment(cacheFile)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, []MountainPass{{ExternalID: "centcols/FR-01-1500", CountryCode: "FR", DepartmentCode: "01", Name: "Col du Colombier", Elevation: 1498}}, mountainPasses)
}

func TestLoadCachedDepartmentMissing(t *testing.T) {
	mountainPasses, found, err := loadCachedDepartment(filepath.Join(t.TempDir(), "does-not-exist.csv"))
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, mountainPasses)
}

func TestLoadCachedDepartmentCorrupt(t *testing.T) {
	cacheFile := filepath.Join(t.TempDir(), "debug_department_01.csv")
	err := os.WriteFile(cacheFile, []byte("Brevet\tcode\tnom\taltitude\nFR-01\n"), 0o644)
	require.NoError(t, err)

	mountainPasses, found, err := loadCachedDepartment(cacheFile)
	require.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, mountainPasses)
}
