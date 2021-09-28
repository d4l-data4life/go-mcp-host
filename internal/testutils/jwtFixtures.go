package testutils

import (
	"fmt"
	"testing"
)

// privKeyEntry is test helper (hence t in params) to generate a valid priv Key entry
//nolint:unparam // we want the "t" to be required here so that the code is used only in tests
func privKeyEntry(t *testing.T, name string, enabled bool) string {
	return `
- name: "` + name + `"
  comment: "generated with: openssl genrsa 2048 -out private.pem"
  enabled: ` + fmt.Sprintf("%t", enabled) + `
  created_at: "unknown"
  author: "Jenkins"
  key: |
    -----BEGIN RSA PRIVATE KEY-----
    MIIEowIBAAKCAQEArUG/lcVDen29RnHK+f1E5UzoXAAMTT5Xatdts7o4+jNwXl7L
    uYSAnmrl50XjyM5fLCog6G+qLz0L6U07EbXB0B/paHuYLlAG9rIYIaAceZYrhMRe
    USx3DL2yIpawa1YR9QYgyHTY2/3sXF+vd/T7JNqBxI/v0vZkhaFCugrWlvAICC1Y
    jQXrjZqRRPl0OsUwZ2kmRvlqvYcVSLEif+uKeNMyplThb9CEQZdjjLMSskYzcmGS
    fPc10ciEDhYR4O5M8vOO5DLeLwj6dw/PTrrslAxrdQqiP4/xyx89ZfFMsxBIBw5J
    eZ0VnQ46Chr5Dy34A/FacA3Sqb0ZEFkmwCTBjwIDAQABAoIBAQCMpb0zhinbPEv0
    7deKzVGqm55dYSSbaCpq72t85YXvhuaHlYjol2oaMElmT9Q0ZWPZZHHGfy+2nWYY
    BLwZCmXF4MIIMZ0+q3Sbu8PfOC0lfwThCNBQMTqLu0rqzU12NS7qrAjc8g5BuIay
    DnNRfCyMpF2IBhj4N1EvMdQLV1UQvYChvuok/oe9xxXPlhb9HCrHhs0WXamhuYmj
    2ZkAPtZ/zM7tzeiHfczx5sUh2BqtkiWDcpezkDhEQhn7C6b3C/2UGfQ8Q1CM3ske
    3D7uLSctvbWH3JNYm0QzRQUgXKYK9xfPsFVv7nxNZyOMtHIrary2Po6PfaGxkGvX
    sdRusDjBAoGBANdzbLNInge/wQKYeUJ7CoOcBWKIpa3kMIAy22wkSAFzE70gCHEn
    7/ppdUGmvHnuzULGQOtXkoHJ3S0TUf4RQ8GYIBCIwD5RkOwj92echkTltUCFsygQ
    b6US4a5WYAg+UNAgSCpzTkj/tGAAtmB4qhN8LHXUOzM0EjChFG/3WJffAoGBAM3d
    Yn9Zq8MjyRViFMgOQzxcv4EfY1tiE2IVJ7skRkI/KBWcpAqg54N3Ft4ih2RzYUob
    e5xPHMu44MqBrDXnk5RGiDI2ph3xvVszTTsFWCtn6PXrh7v8OTYovsiww/aN10/p
    Rn7zz7oSAKUizyU6tdu6xMOW7GE8lsI/S70aycxRAoGAJqdwwyGuKJnAmSSd7M2C
    b2ZYmPsHLpGYGggF0fsYaBorWm0a1qJhrb2p6eNuQToU3XwQPajyghKjeejTdw/F
    5j/S0OSYCRY9OACj7JTqigXkZPUX1YJNZYJjtxGMHS6A9TY1fFg/nV0zEV5PWjOL
    3/8RQvqWvHMFKHBd6FCqNmUCgYAb2/rpaxQwi1Y6G5TeYfe9YnvUGJBUnJgs7Nn8
    nHMZofxluFYGzjGme+ZPV3LlKCwhYEjBJX+rHjDlltjcTqONLGJgET83zDAo+G9a
    LmX5Mc24AhDTYtXHO4peFHXglt9thA8zPQF+l9MYhfZsfl6ABu173p/MpOtuDCzO
    waJPkQKBgGNWifCTY+rfDyzZOO50jefGXALd6rhMscfGED+gwyfFfHQxdiutDLnI
    VPd3tSZu2RU0c3a5wEEFqlJAl07VkVLg96mTKlW7dJzvvS3eqXR3v56f3+MSvTrO
    ZBQeOldZwwPpJEnP4bhDlAOqFtffc3JmvsczOOhYVDkLduuUgVUc
    -----END RSA PRIVATE KEY-----`
}

// pubKeyEntry is test helper (hence t in params) to generate a valid pub Key entry
//nolint:unparam // we want the "t" to be required here so that the code is used only in tests
func pubKeyEntry(t *testing.T, name string) string {
	return `
- name: "` + name + `"
  comment: "generated with: openssl rsa -in private.pem -pubout -outform PEM -out public.pem"
  not_before: 2020-01-01
  not_after: 2022-01-01
  key: |
    -----BEGIN PUBLIC KEY-----
    MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEArUG/lcVDen29RnHK+f1E
    5UzoXAAMTT5Xatdts7o4+jNwXl7LuYSAnmrl50XjyM5fLCog6G+qLz0L6U07EbXB
    0B/paHuYLlAG9rIYIaAceZYrhMReUSx3DL2yIpawa1YR9QYgyHTY2/3sXF+vd/T7
    JNqBxI/v0vZkhaFCugrWlvAICC1YjQXrjZqRRPl0OsUwZ2kmRvlqvYcVSLEif+uK
    eNMyplThb9CEQZdjjLMSskYzcmGSfPc10ciEDhYR4O5M8vOO5DLeLwj6dw/PTrrs
    lAxrdQqiP4/xyx89ZfFMsxBIBw5JeZ0VnQ46Chr5Dy34A/FacA3Sqb0ZEFkmwCTB
    jwIDAQAB
    -----END PUBLIC KEY-----`
}
