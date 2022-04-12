package qldb_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go/service/qldb"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	tfqldb "github.com/hashicorp/terraform-provider-aws/internal/service/qldb"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func TestAccQLDBStream_basic(t *testing.T) {
	var v qldb.JournalKinesisStreamDescription
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_qldb_stream.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { acctest.PreCheck(t); acctest.PreCheckPartitionHasService(qldb.EndpointsID, t) },
		ErrorCheck:   acctest.ErrorCheck(t, qldb.EndpointsID),
		Providers:    acctest.Providers,
		CheckDestroy: testAccCheckStreamDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccStreamConfig(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckStreamExists(resourceName, &v),
					acctest.MatchResourceAttrRegionalARN(resourceName, "arn", "qldb", regexp.MustCompile(`stream/.+`)),
					resource.TestCheckResourceAttr(resourceName, "exclusive_end_time", ""),
					resource.TestCheckResourceAttrSet(resourceName, "inclusive_start_time"),
					resource.TestCheckResourceAttr(resourceName, "kinesis_configuration.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "kinesis_configuration.0.aggregation_enabled", "true"),
					resource.TestCheckResourceAttrSet(resourceName, "kinesis_configuration.0.stream_arn"),
					resource.TestCheckResourceAttrSet(resourceName, "ledger_name"),
					resource.TestCheckResourceAttrSet(resourceName, "role_arn"),
					resource.TestCheckResourceAttr(resourceName, "stream_name", rName),
					resource.TestCheckResourceAttr(resourceName, "tags.%", "0"),
				),
			},
		},
	})
}

func testAccCheckStreamExists(n string, v *qldb.JournalKinesisStreamDescription) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No QLDB Stream ID is set")
		}

		conn := acctest.Provider.Meta().(*conns.AWSClient).QLDBConn

		output, err := tfqldb.FindStream(context.TODO(), conn, rs.Primary.Attributes["ledger_name"], rs.Primary.ID)

		if err != nil {
			return err
		}

		*v = *output

		return nil
	}
}

func testAccCheckStreamDestroy(s *terraform.State) error {
	conn := acctest.Provider.Meta().(*conns.AWSClient).QLDBConn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_qldb_stream" {
			continue
		}

		_, err := tfqldb.FindStream(context.TODO(), conn, rs.Primary.Attributes["ledger_name"], rs.Primary.ID)

		if tfresource.NotFound(err) {
			continue
		}

		if err != nil {
			return err
		}

		return fmt.Errorf("QLDB Stream %s still exists", rs.Primary.ID)
	}

	return nil
}

func testAccStreamConfig(rName string) string {
	return fmt.Sprintf(`
resource "aws_qldb_ledger" "test" {
  name                = %[1]q
  permissions_mode    = "ALLOW_ALL"
  deletion_protection = false
}

resource "aws_kinesis_stream" "test" {
  name             = %[1]q
  shard_count      = 1
  retention_period = 24
}

resource "aws_iam_role" "test" {
  name = %[1]q

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Sid    = ""
      Principal = {
        Service = "qldb.amazonaws.com"
      }
    },]
  })

  inline_policy {
    name = "test-qldb-policy"
    policy = jsonencode({
      Version = "2012-10-17"
      Statement = [{
        Action = [
          "kinesis:PutRecord*",
          "kinesis:DescribeStream",
          "kinesis:ListShards",
        ]
        Effect   = "Allow"
        Resource = aws_kinesis_stream.test.arn
      }]
    })
  }
}

resource "aws_qldb_stream" "test" {
  stream_name          = %[1]q
  ledger_name          = aws_qldb_ledger.test.id
  inclusive_start_time = "2021-01-01T00:00:00Z"
  role_arn             = aws_iam_role.test.arn

  kinesis_configuration {
    stream_arn = aws_kinesis_stream.test.arn
  }
}
`, rName)
}
