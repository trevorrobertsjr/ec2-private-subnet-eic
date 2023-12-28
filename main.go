// When using native packages, must specify a region in the config.
// pulumi config set aws-native:region us-east-1

package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	ec2native "github.com/pulumi/pulumi-aws-native/sdk/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

// **************************************************
// CREATE THE VPC
// **************************************************
		// Create a new VPC
		vpc, err := ec2.NewVpc(ctx, "myVpc", &ec2.VpcArgs{
			CidrBlock: pulumi.String("14.0.0.0/16"),
		})
		if err != nil {
			return err
		}

		subnet1, err := ec2.NewSubnet(ctx, "mySubnetOne", &ec2.SubnetArgs{
			AvailabilityZone: pulumi.String("us-east-1a"),
			VpcId:     vpc.ID(),
			CidrBlock: pulumi.String("14.0.1.0/24"),
		})
		if err != nil {
			return err
		}

		subnet2, err := ec2.NewSubnet(ctx, "mySubnetTwo", &ec2.SubnetArgs{
			AvailabilityZone: pulumi.String("us-east-1b"),
			VpcId:     vpc.ID(),
			CidrBlock: pulumi.String("14.0.2.0/24"),
		})
		if err != nil {
			return err
		}

// **************************************************
// CREATE THE SECURITY GROUPS
// **************************************************
		// Create a security group for compute resources to access the database.
		computeSg, err := ec2.NewSecurityGroup(ctx, "computeAccessDBSecurityGroup", &ec2.SecurityGroupArgs{
			VpcId: vpc.ID(),
		})
		if err != nil {
			return err
		}
		// Create a security group for compute resources to access the database.
		eicSg, err := ec2.NewSecurityGroup(ctx, "ec2InstanceConnectSecurityGroup", &ec2.SecurityGroupArgs{
			VpcId: vpc.ID(),
		})
		if err != nil {
			return err
		}

// **************************************************
// CREATE AN EC2 INSTANCE 
// **************************************************
		// Allow access from EC2 Instance Connect Endpoint on TCP 22.
		_, err = ec2.NewSecurityGroupRule(ctx, "ec2InstanceConnectEndpointToEc2SecurityGroupRule", &ec2.SecurityGroupRuleArgs{
			Type:            pulumi.String("ingress"),
			FromPort:        pulumi.Int(22),
			ToPort:          pulumi.Int(22),
			Protocol:        pulumi.String("tcp"),
			SecurityGroupId: computeSg.ID(),
			SourceSecurityGroupId: eicSg.ID(),
		})
		if err != nil {
			return err
		}
		// Create new IAM role for EC2 Instance permissions to be granted later
		role, err := iam.NewRole(ctx, "myInstanceRole", &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(`{
				"Version": "2012-10-17",
				"Statement": [{
					"Action": "sts:AssumeRole",
					"Effect": "Allow",
					"Principal": {
						"Service": "ec2.amazonaws.com"
					}
				}]
			}`),
		})
		if err != nil {
			return err
		}

		// Attach AmazonEC2RoleforSSM policy to role
		_, err = iam.NewRolePolicyAttachment(ctx, "myRolePolicyAttachment", &iam.RolePolicyAttachmentArgs{
			Role:      role.Name,
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"),
		})
		if err != nil {
			return err
		}

		// Create an IAM Instance Profile for the EC2 instance.
		instanceProfile, err := iam.NewInstanceProfile(ctx, "myInstanceProfile", &iam.InstanceProfileArgs{
			Role: role.Name,
		})
		if err != nil {
			return err
		}

		// Create the EC2 instance in the private subnet with the role
		ec2Instance, err := ec2.NewInstance(ctx, "myInstance", &ec2.InstanceArgs{
			Ami:                   pulumi.String("ami-02cd6549baea35b55"), // Replace with actual Amazon Linux 2023 AMI ID for us-east-1
			// Ami:					pulumi.String("ami-0c7217cdde317cfec"),
			InstanceType:          pulumi.String("t4g.nano"),
			SubnetId:              subnet1.ID(),
			VpcSecurityGroupIds: pulumi.StringArray{computeSg.ID()},
			IamInstanceProfile:    instanceProfile.Name, // Attach the IAM role to the EC2 instance
			AssociatePublicIpAddress: pulumi.Bool(false), // Private subnet implies no public IP
			// Security Group, VPC ID, and other required resource arguments are assumed
			// to be handled within a proper security group or other configuration
		})
		if err != nil {
			return err
		}


// **********************************************************************
// CREATE THE EC2 INSTANCE CONNECT ENDPOINT FOR PRIVATE EC2 SSH
// **********************************************************************
		// IMPORTANT: Allow outbound traffic from EC2 Instance Connect Endpoint on TCP 22.
		_, err = ec2.NewSecurityGroupRule(ctx, "ec2InstanceConnectEndpointEgressToEc2SecurityGroupRule", &ec2.SecurityGroupRuleArgs{
			Type:            pulumi.String("egress"),
			FromPort:        pulumi.Int(22),
			ToPort:          pulumi.Int(22),
			Protocol:        pulumi.String("tcp"),
			SecurityGroupId: eicSg.ID(),
			SourceSecurityGroupId: computeSg.ID(),
		})
		if err != nil {
			return err
		}

		_, err = ec2native.NewInstanceConnectEndpoint(ctx, "instanceConnectEndpoint", &ec2native.InstanceConnectEndpointArgs{
			SubnetId:          subnet2.ID(), // Reference to the private subnet
			SecurityGroupIds:  pulumi.StringArray{eicSg.ID()}, // Reference to the security group
			PreserveClientIp:  pulumi.Bool(false),
		})
		if err != nil {
			return err
		}


// **************************************************
// CREATE THE OUTPUTS
// **************************************************
		// Export the cluster's endpoint and ID.
		ctx.Export("ec2 ID", ec2Instance.ID())

		return nil
	})
}