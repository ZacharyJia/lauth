package main

import (
	"fmt"
	"net"
	"net/url"
	"os"

	"github.com/alecthomas/kingpin"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

var (
	app = kingpin.New("Ldapin", "The simple OpenID Provider for LDAP like a ActiveDirectory.")

	Issuer  = app.Flag("issuer", "Issuer URL.").Envar("LDAPIN_ISSUER").PlaceHolder(DefaultConfig.Issuer.String()).URL()
	Listen  = app.Flag("listen", "Listen address and port. In default, use same port as Issuer URL. This option can't use when auto generate TLS cert.").Envar("LDAPIN_LISTEN").TCP()
	SignKey = app.Flag("sign-key", "RSA private key for signing to token. If omit this, automate generate key for one time use.").Envar("LDAPIN_SIGN_KEY").PlaceHolder("FILE").File()

	TLSCertFile = app.Flag("tls-cert", "Cert file for TLS encryption.").Envar("LDAPIN_TLS_CERT").PlaceHolder("FILE").ExistingFile()
	TLSKeyFile  = app.Flag("tls-key", "Key file for TLS encryption.").Envar("LDAPIN_TLS_KEY").PlaceHolder("FILE").ExistingFile()

	AuthzEndpoint    = app.Flag("authz-endpoint", "Path to authorization endpoint.").Envar("LDAPIN_AUTHz_ENDPOINT").PlaceHolder(DefaultConfig.Endpoints.Authz).String()
	TokenEndpoint    = app.Flag("token-endpoint", "Path to token endpoint.").Envar("LDAPIN_TOKEN_ENDPOINT").PlaceHolder(DefaultConfig.Endpoints.Token).String()
	UserinfoEndpoint = app.Flag("userinfo-endpoint", "Path to userinfo endpoint.").Envar("LDAPIN_USERINFO_ENDPOINT").PlaceHolder(DefaultConfig.Endpoints.Userinfo).String()
	JwksEndpoint     = app.Flag("jwks-uri", "Path to jwks uri.").Envar("LDAPIN_JWKS_URI").PlaceHolder(DefaultConfig.Endpoints.Jwks).String()

	CodeTTL  = app.Flag("code-ttl", "TTL for code.").Envar("LDAPIN_CODE_TTL").PlaceHolder("10m").String()
	TokenTTL = app.Flag("token-ttl", "TTL for access_token and id_token.").Envar("LDAPIN_TOKEN_TTL").PlaceHolder("7d").String()
	SSOTTL   = app.Flag("sso-ttl", "TTL for single sign-on.").Envar("LDAPIN_SSO_TTL").PlaceHolder("14d").String()

	LdapAddress     = app.Flag("ldap", "URL of LDAP server like \"ldap://USER_DN:PASSWORD@ldap.example.com\".").Envar("LDAP_ADDRESS").PlaceHolder("ADDRESS").Required().URL()
	LdapBaseDN      = app.Flag("ldap-base-dn", "The base DN for search user account in LDAP like \"OU=somewhere,DC=example,DC=local\".").Envar("LDAP_BASE_DN").PlaceHolder("DN").Required().String() // TODO: make it automate set same OU as bind user if omit.
	LdapIDAttribute = app.Flag("ldap-id-attribute", "ID attribute name in LDAP.").Envar("LDAP_ID_ATTRIBUTE").Default("sAMAccountName").String()
	LdapDisableTLS  = app.Flag("ldap-disable-tls", "Disable use TLS when connect to LDAP server. THIS IS INSECURE.").Envar("LDAP_DISABLE_TLS").Bool()

	LoginPage = app.Flag("login-page", "Templte file for login page.").Envar("LDAPIN_LOGIN_PAGE").PlaceHolder("FILE").File()
	ErrorPage = app.Flag("error-page", "Templte file for error page.").Envar("LDAPIN_ERROR_PAGE").PlaceHolder("FILE").File()

	Config  = app.Flag("config", "Load options from YAML file.").Envar("LDAPIN_CONFIG").PlaceHolder("FILE").File()
	Verbose = app.Flag("verbose", "Enable debug mode.").Envar("LDAPIN_VERBOSE").Bool()
)

func DecideListenAddress(issuer *url.URL, listen *net.TCPAddr) string {
	if listen != nil {
		return listen.String()
	}

	if issuer.Port() != "" {
		return fmt.Sprintf(":%s", issuer.Port())
	}

	if issuer.Scheme == "https" {
		return ":443"
	}
	return ":80"
}

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	var codeExpiresIn, tokenExpiresIn Duration
	var err error
	if *CodeTTL != "" {
		codeExpiresIn, err = ParseDuration(*CodeTTL)
		app.FatalIfError(err, "--code-ttl")
	}
	if *TokenTTL != "" {
		tokenExpiresIn, err = ParseDuration(*TokenTTL)
		app.FatalIfError(err, "--token-ttl")
	}

	if *TLSCertFile != "" && *TLSKeyFile == "" {
		app.Fatalf("--tls-key is required when set --tls-cert")
	} else if *TLSCertFile == "" && *TLSKeyFile != "" {
		app.Fatalf("--tls-cert is required when set --tls-key")
	}
	if *TLSCertFile != "" && *TLSKeyFile != "" && (*Issuer).Scheme != "https" {
		app.Fatalf("Please set https URL for --issuer when use TLS.")
	}

	if *Verbose {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	ldapUser := (*LdapAddress).User.Username()
	ldapPassword, _ := (*LdapAddress).User.Password()
	if ldapUser == "" && ldapPassword == "" {
		app.Fatalf("--ldap is must be has user and password information.")
		return
	}

	connector := SimpleLDAPConnector{
		ServerURL:   *LdapAddress,
		User:        ldapUser,
		Password:    ldapPassword,
		IDAttribute: *LdapIDAttribute,
		BaseDN:      *LdapBaseDN,
		DisableTLS:  *LdapDisableTLS,
	}
	_, err = connector.Connect()
	app.FatalIfError(err, "failed to connect LDAP server")

	var jwt JWTManager
	if *SignKey != nil {
		jwt, err = NewJWTManagerFromFile(*SignKey)
		app.FatalIfError(err, "failed to read private key for sign")
	} else {
		jwt, err = GenerateJWTManager()
		app.FatalIfError(err, "failed to generate private key for sign")
	}

	conf := DefaultConfig
	if *Config != nil {
		loaded, err := LoadConfig(*Config)
		app.FatalIfError(err, "failed to load config file")
		conf.Override(loaded)
	}
	conf.Override(&LdapinConfig{
		Issuer: (*URL)(*Issuer),
		Listen: (*TCPAddr)(*Listen),
		TTL: TTLConfig{
			Code:  codeExpiresIn,
			Token: tokenExpiresIn,
		},
		Endpoints: EndpointConfig{
			Authz:    *AuthzEndpoint,
			Token:    *TokenEndpoint,
			Userinfo: *UserinfoEndpoint,
			Jwks:     *JwksEndpoint,
		},
	})
	api := &LdapinAPI{
		Connector:  connector,
		JWTManager: jwt,
		Config:     conf,
	}

	tmpl, err := loadPageTemplate(*LoginPage, *ErrorPage)
	app.FatalIfError(err, "failed to load template")
	router.SetHTMLTemplate(tmpl)

	router.Use(gzip.Gzip(gzip.DefaultCompression))

	api.SetRoutes(router)
	api.SetErrorRoutes(router)

	addr := DecideListenAddress((*url.URL)(conf.Issuer), (*net.TCPAddr)(conf.Listen))
	if *TLSCertFile != "" {
		err = router.RunTLS(addr, *TLSCertFile, *TLSKeyFile)
		app.FatalIfError(err, "failed to start server")
	} else {
		err = router.Run(addr)
		app.FatalIfError(err, "failed to start server")
	}
}
