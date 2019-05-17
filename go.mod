module github.com/weaveworks/service

go 1.12

require (
	cloud.google.com/go v0.0.0-20180118014213-eb1cc5f3c0a9
	github.com/ExpansiveWorlds/instrumentedsql v0.0.0-20170424151410-55151537c3a7
	github.com/FrenchBen/goketo v0.0.0-20180808185808-f48837e0040f
	github.com/Masterminds/semver v1.4.2 // indirect
	github.com/Masterminds/squirrel v0.0.0-20170825200431-a6b93000bd21
	github.com/armon/go-metrics v0.0.0-20180713145231-3c58d8115a78 // indirect
	github.com/armon/go-proxyproto v0.0.0-20180920091222-0a8c12a32b20
	github.com/armon/go-socks5 v0.0.0-20160902184237-e75332964ef5
	github.com/aws/aws-sdk-go v1.12.63
	github.com/badoux/checkmail v0.0.0-20170203135005-d0a759655d62
	github.com/beorn7/perks v0.0.0-20160804104726-4c0e84591b9a // indirect
	github.com/bluele/gcache v0.0.0-20170906030344-6748215c018e
	github.com/certifi/gocertifi v0.0.0-20170727155124-3fd9e1adb12b
	github.com/cespare/xxhash v1.0.0 // indirect
	github.com/cihub/seelog v0.0.0-20151216151435-d2c6e5aa9fbf // indirect
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd // indirect
	github.com/davecgh/go-spew v1.1.0 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/docker/distribution v0.0.0-20171011171712-7484e51bf6af // indirect
	github.com/docker/libtrust v0.0.0-20160708172513-aabc10ec26b7 // indirect
	github.com/dukex/mixpanel v0.0.0-20170510165255-53bfdf679eec
	github.com/fluent/fluent-logger-golang v1.2.1
	github.com/franela/goreq v0.0.0-20171204163338-bcd34c9993f8 // indirect
	github.com/go-ini/ini v1.32.0 // indirect
	github.com/go-kit/kit v0.6.0
	github.com/go-logfmt/logfmt v0.3.0 // indirect
	github.com/go-stack/stack v1.6.0 // indirect
	github.com/gogo/googleapis v1.1.0 // indirect
	github.com/gogo/protobuf v1.2.1
	github.com/gogo/status v1.0.3 // indirect
	github.com/golang/gddo v0.0.0-20171013234608-2fa06788d5bf // indirect
	github.com/golang/mock v1.2.0
	github.com/golang/protobuf v1.3.1
	github.com/google/go-github v0.0.0-20170202165540-59aa6eea1c58
	github.com/google/go-querystring v0.0.0-20170111101155-53e6ce116135 // indirect
	github.com/google/uuid v0.0.0-20161128191214-064e2069ce9c
	github.com/googleapis/gax-go v2.0.0+incompatible // indirect
	github.com/gorilla/context v0.0.0-20160226214623-1ea25387ff6f // indirect
	github.com/gorilla/mux v1.6.2
	github.com/gorilla/securecookie v0.0.0-20160422134519-667fe4e3466a
	github.com/gorilla/websocket v1.2.0
	github.com/grpc-ecosystem/grpc-opentracing v0.0.0-20180507213350-8e809c8a8645
	github.com/hashicorp/errwrap v0.0.0-20180715044906-d6c0cd880357 // indirect
	github.com/hashicorp/go-cleanhttp v0.0.0-20171218145408-d5fe4b57a186 // indirect
	github.com/hashicorp/go-immutable-radix v0.0.0-20180129170900-7f3cd4390caa // indirect
	github.com/hashicorp/go-multierror v0.0.0-20180717150148-3d5d8f294aa0
	github.com/hashicorp/golang-lru v0.0.0-20180201235237-0fb14efe8c47 // indirect
	github.com/jinzhu/now v0.0.0-20170212112655-d939ba741945 // indirect
	github.com/jmespath/go-jmespath v0.0.0-20160202185014-0b12d6b521d8 // indirect
	github.com/jmoiron/sqlx v0.0.0-20170430194603-d9bd385d68c0
	github.com/jordan-wright/email v0.0.0-20160301001728-a62870b0c368
	github.com/justinas/nosurf v0.0.0-20170823093306-cbe5fdb4a426
	github.com/k-sone/critbitgo v1.2.0 // indirect
	github.com/kr/logfmt v0.0.0-20140226030751-b84e30acd515 // indirect
	github.com/kr/pretty v0.0.0-20160823170715-cfb55aafdaf3 // indirect
	github.com/kr/text v0.0.0-20160504234017-7cafcd837844 // indirect
	github.com/lann/builder v0.0.0-20150808151131-f22ce00fd939 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/lib/pq v0.0.0-20170918175043-23da1db4f16d
	github.com/mattn/go-colorable v0.0.9 // indirect
	github.com/mattn/go-isatty v0.0.2 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.0 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b
	github.com/microcosm-cc/bluemonday v1.0.1
	github.com/miekg/dns v1.0.8 // indirect
	github.com/mitchellh/copystructure v0.0.0-20170525013902-d23ffcb85de3
	github.com/mitchellh/reflectwalk v0.0.0-20170726202117-63d60e9d0dbc // indirect
	github.com/mwitkow/go-grpc-middleware v0.0.0-20170825075817-645b33ed7ba8
	github.com/nats-io/go-nats v1.3.0
	github.com/nats-io/nuid v1.0.0 // indirect
	github.com/nightlyone/lockfile v0.0.0-20170804114028-6a197d5ea611
	github.com/oklog/ulid v0.3.0 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opentracing-contrib/go-stdlib v0.0.0-20190510164024-2b2d2700a3b7
	github.com/opentracing/opentracing-go v1.0.2
	github.com/opsgenie/opsgenie-go-sdk v0.0.0-20180124072621-e5f3103b77a5
	github.com/philhofer/fwd v1.0.0 // indirect
	github.com/pkg/errors v0.8.0
	github.com/pkg/term v0.0.0-20180730021639-bffc007b7fd5 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v0.0.0-20180203142815-9bb6ab929dcb
	github.com/prometheus/client_model v0.0.0-20170216185247-6f3806018612 // indirect
	github.com/prometheus/common v0.0.0-20170908161822-2f17f4a9d485
	github.com/prometheus/procfs v0.0.0-20170703101242-e645f4e5aaa8 // indirect
	github.com/prometheus/prometheus v2.2.1+incompatible
	github.com/prometheus/tsdb v0.0.0-20180315191547-195bc0d286b0 // indirect
	github.com/robfig/cron v1.0.0
	github.com/ryanuber/go-glob v0.0.0-20160226084822-572520ed46db // indirect
	github.com/satori/go.uuid v1.1.0
	github.com/segmentio/analytics-go v0.0.0-20180906214725-c541109ba065
	github.com/segmentio/backo-go v0.0.0-20160424052352-204274ad699c // indirect
	github.com/sendgrid/rest v2.4.1+incompatible // indirect
	github.com/sendgrid/sendgrid-go v3.4.1+incompatible
	github.com/sercand/kuberesolver v1.0.0 // indirect
	github.com/shurcooL/sanitized_anchor_name v0.0.0-20170918181015-86672fcb3f95 // indirect
	github.com/sirupsen/logrus v1.0.3
	github.com/spf13/pflag v1.0.0 // indirect
	github.com/stretchr/testify v1.1.4
	github.com/tinylib/msgp v1.0.2 // indirect
	github.com/tylerb/graceful v1.2.15
	github.com/uber/jaeger-client-go v2.14.0+incompatible
	github.com/uber/jaeger-lib v1.5.0 // indirect
	github.com/ugorji/go v0.0.0-20170918222552-54210f4e076c // indirect
	github.com/weaveworks/billing-client v0.0.0-20171116141500-967593a9e8c8
	github.com/weaveworks/blackfriday v0.0.0-20151110051855-0b647d0506a6
	github.com/weaveworks/common v0.0.0-20180919191744-2a0cb6145a2f
	github.com/weaveworks/flux v0.0.0-20190321171346-48966b9d6191
	github.com/weaveworks/launcher v0.0.0-20180220152040-985a36f4b7b4
	github.com/weaveworks/promrus v1.2.0 // indirect
	github.com/weaveworks/ps v0.0.0-20160725183535-70d17b2d6f76 // indirect
	github.com/weaveworks/scope v0.0.0-20180725094325-058eedc7f14a
	github.com/whilp/git-urls v0.0.0-20160530060445-31bac0d230fa // indirect
	github.com/xtgo/uuid v0.0.0-20140804021211-a0b114877d4c // indirect
	golang.org/x/crypto v0.0.0-20170916190215-7d9177d70076
	golang.org/x/net v0.0.0-20170915142106-8351a756f30f
	golang.org/x/oauth2 v0.0.0-20170912212905-13449ad91cb2
	golang.org/x/sync v0.0.0-20171101214715-fd80eb99c8f6 // indirect
	golang.org/x/sys v0.0.0-20170919001338-b6e1ae216436 // indirect
	golang.org/x/text v0.0.0-20170915090833-1cbadb444a80 // indirect
	golang.org/x/time v0.0.0-20170927054726-6dc17368e09b
	golang.org/x/tools v0.0.0-20180221164845-07fd8470d635
	google.golang.org/api v0.0.0-20180104000315-251495678c78
	google.golang.org/appengine v1.0.0 // indirect
	google.golang.org/genproto v0.0.0-20170918111702-1e559d0a00ee // indirect
	google.golang.org/grpc v1.13.0
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/go-playground/webhooks.v3 v3.3.0
	gopkg.in/gomail.v2 v2.0.0-20150902115704-41f357289737
	gopkg.in/h2non/gock.v1 v1.0.6
	gopkg.in/mattes/migrate.v1 v1.3.2
	gopkg.in/yaml.v2 v2.0.0-20180102123842-c95af922eae6
)
