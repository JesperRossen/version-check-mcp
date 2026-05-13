package pypi

// projectURL returns the PyPI JSON API URL for the given project.
// Per D-PYPI-01: https://pypi.org/pypi/{project}/json
// The project name is passed through as-is — PyPI normalises case server-side;
// there is no scoped-package equivalent that requires percent-encoding.
func projectURL(project string) string {
	return "https://pypi.org/pypi/" + project + "/json"
}
