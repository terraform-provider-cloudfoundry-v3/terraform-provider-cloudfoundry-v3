module github.com/terraform-providers/terraform-provider-cloudfoundry

go 1.14

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20200131002437-cf55d5288a48 // indirect
	code.cloudfoundry.org/cfnetworking-cli-api v0.0.0-20190103195135-4b04f26287a6
	code.cloudfoundry.org/cli v0.0.0-20201105145344-e3856be8d239
	code.cloudfoundry.org/cli-plugin-repo v0.0.0-20201105212819-c5f33c989f5d // indirect
	code.cloudfoundry.org/rfc5424 v0.0.0-20201103192249-000122071b78 // indirect
	github.com/Sirupsen/logrus v1.7.0 // indirect
	github.com/aws/aws-sdk-go v1.31.9 // indirect
	github.com/bmatcuk/doublestar v1.3.3 // indirect
	github.com/charlievieth/fs v0.0.1 // indirect
	github.com/cloudfoundry-community/go-uaa v0.3.1
	github.com/cloudfoundry/bosh-cli v6.4.1+incompatible // indirect
	github.com/cloudfoundry/bosh-utils v0.0.0-20201107100218-f523638849f6 // indirect
	github.com/cppforlife/go-patch v0.2.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/docker/docker v1.13.1 // indirect
	github.com/fatih/color v1.10.0 // indirect
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/hashicorp/hcl/v2 v2.6.0 // indirect
	github.com/hashicorp/terraform-plugin-sdk/v2 v2.2.0
	github.com/konsorten/go-windows-terminal-sequences v1.0.3 // indirect
	github.com/kr/pty v1.1.8 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/moby/moby v1.13.1 // indirect
	github.com/sabhiram/go-gitignore v0.0.0-20180611051255-d3107576ba94 // indirect
	github.com/sirupsen/logrus v1.7.0 // indirect
	github.com/vito/go-interact v1.0.0 // indirect
	github.com/zclconf/go-cty v1.5.1 // indirect
	golang.org/x/net v0.0.0-20201031054903-ff519b6c9102 // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sys v0.0.0-20201107080550-4d91cf3a1aaf // indirect
	golang.org/x/text v0.3.4 // indirect
	google.golang.org/genproto v0.0.0-20201106154455-f9bfe239b0ba // indirect
	google.golang.org/grpc v1.33.2 // indirect
	gopkg.in/cheggaaa/pb.v1 v1.0.28 // indirect
	gopkg.in/yaml.v2 v2.3.0
)

// something in the dep tree using wrong import path
replace github.com/cloudfoundry/cli-plugin-repo => code.cloudfoundry.org/cli-plugin-repo v0.0.0-20200304195157-af98c4be9b85

// this repo got renamed, but a dep still imports via old name
replace github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.4.2

// something is missing a go.mod file
// replace google.golang.org/grpc => google.golang.org/grpc v1.27.1
