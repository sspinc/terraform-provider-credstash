# Terraform provider for credstash secrets

[![CircleCI](https://circleci.com/gh/sspinc/terraform-provider-credstash.svg?style=svg)](https://circleci.com/gh/sspinc/terraform-provider-credstash)

Read secrets stored with [credstash][credstash].

## Install

1. [Download the binary for your platform][provider_binary]
2. Create the terraform plugin directory

        $ mkdir ~/.terraform.d/plugins

3. Copy the provider binary to the terraform plugin directory

         $ cp /path/to/terraform-provider-credstash ~/.terraform.d/plugins/terraform-provider-credstash_v0.3.0

4. Profit

### From source

    $ git clone https://github.com/sspinc/terraform-provider-credstash.git
    $ cd /path/to/terraform-provider-credstash
    $ make install

## Usage

```hcl
provider "credstash" {
    table  = "credential-store"
    region = "us-east-1"
}

data "credstash_secret" "rds_password" {
    name = "rds_password"
}

data "credstash_secret" "my_secret" {
    name    = "some_secret"
    version = "0000000000000000001"
}

resource "aws_db_instance" "postgres" {
    password = "${data.credstash_secret.rds_password.value}"

    # other important attributes
}
```

You can override the table on a per data source basis:

```hcl
data "credstash_secret" "my_secret" {
    table   = "some_table"
    name    = "some_secret"
    version = "0000000000000000001"
}
```

## AWS credentials

AWS credentials are not directly set. Use one of the methods discussed
[here][awscred].

You can set a specific profile to use:

```hcl
provider "credstash" {
    region  = "us-east-1"
    profile = "my-profile"
}
```

## IAM role 

Use IAM role attached to AWS instance and assume to other role

```hcl
provider "credstash" {
  table  = "credstash-table"
  region = "ap-southeast-2"

  assume_role {
    role_arn = "role_arn"
    session_name = "session_name"
    external_id = "external_id"
  }
}
```


## Development

For dependency management Go modules are used thus you will need go 1.11+

1. Clone the repo `git clone https://github.com/sspinc/terraform-provider-credstash.git`
2. Run `make test` to run all tests

## Contributing

1. Fork the project and clone it locally
2. Open a feature brach `git checkout -b my-awesome-feature`
3. Make your changes
4. Commit your changes
5. Push your changes
6. Open a pull request

[credstash]: https://github.com/fugue/credstash
[awscred]: https://github.com/aws/aws-sdk-go#configuring-credentials
[provider_binary]: https://github.com/sspinc/terraform-provider-credstash/releases/latest
