package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/versus-control/ai-infrastructure-agent/pkg/types"

	"github.com/sirupsen/logrus"
)

// ========== EC2 Instance Management Methods ==========

// CreateEC2Instance creates a new EC2 instance
func (c *Client) CreateEC2Instance(ctx context.Context, params CreateInstanceParams) (*types.AWSResource, error) {
	c.logger.WithFields(logrus.Fields{
		"imageId":         params.ImageID,
		"instanceType":    params.InstanceType,
		"keyName":         params.KeyName,
		"securityGroupId": params.SecurityGroupID,
		"subnetId":        params.SubnetID,
		"name":            params.Name,
	}).Info("CreateEC2Instance called with parameters")

	input := &ec2.RunInstancesInput{
		ImageId:      aws.String(params.ImageID),
		InstanceType: ec2types.InstanceType(params.InstanceType),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
	}

	if params.KeyName != "" {
		input.KeyName = aws.String(params.KeyName)
	}

	if params.SecurityGroupID != "" {
		input.SecurityGroupIds = []string{params.SecurityGroupID}
		c.logger.WithField("securityGroupIds", input.SecurityGroupIds).Debug("Security group IDs set")
	}

	if params.SubnetID != "" {
		input.SubnetId = aws.String(params.SubnetID)
		c.logger.WithField("subnetId", params.SubnetID).Debug("Subnet ID set")
	}

	// Add tag specifications during creation if name is provided
	if params.Name != "" {
		input.TagSpecifications = []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeInstance,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(params.Name),
					},
				},
			},
		}
	}

	result, err := c.ec2.RunInstances(ctx, input)
	if err != nil {
		c.logger.WithError(err).Error("Failed to create EC2 instance")
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	if len(result.Instances) == 0 {
		return nil, fmt.Errorf("no instances created")
	}

	instance := result.Instances[0]
	resource := c.convertEC2Instance(instance)

	c.logger.WithField("instanceId", *instance.InstanceId).Info("EC2 instance created successfully")
	return resource, nil
}

// StartEC2Instance starts a stopped EC2 instance
func (c *Client) StartEC2Instance(ctx context.Context, instanceID string) error {
	input := &ec2.StartInstancesInput{
		InstanceIds: []string{instanceID},
	}

	_, err := c.ec2.StartInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to start instance %s: %w", instanceID, err)
	}

	c.logger.WithField("instanceId", instanceID).Info("EC2 instance start initiated")
	return nil
}

// StopEC2Instance stops a running EC2 instance
func (c *Client) StopEC2Instance(ctx context.Context, instanceID string) error {
	input := &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	}

	_, err := c.ec2.StopInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to stop instance %s: %w", instanceID, err)
	}

	c.logger.WithField("instanceId", instanceID).Info("EC2 instance stop initiated")
	return nil
}

// TerminateEC2Instance terminates an EC2 instance
func (c *Client) TerminateEC2Instance(ctx context.Context, instanceID string) error {
	input := &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	}

	_, err := c.ec2.TerminateInstances(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to terminate instance %s: %w", instanceID, err)
	}

	c.logger.WithField("instanceId", instanceID).Info("EC2 instance termination initiated")
	return nil
}

// convertEC2Instance converts an EC2 instance to our internal resource representation
func (c *Client) convertEC2Instance(instance ec2types.Instance) *types.AWSResource {
	tags := make(map[string]string)
	for _, tag := range instance.Tags {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}

	details := map[string]interface{}{
		"instanceType":     string(instance.InstanceType),
		"imageId":          aws.ToString(instance.ImageId),
		"launchTime":       instance.LaunchTime,
		"privateIpAddress": aws.ToString(instance.PrivateIpAddress),
		"publicIpAddress":  aws.ToString(instance.PublicIpAddress),
		"subnetId":         aws.ToString(instance.SubnetId),
		"vpcId":            aws.ToString(instance.VpcId),
	}

	if instance.Placement != nil {
		details["availabilityZone"] = aws.ToString(instance.Placement.AvailabilityZone)
	}

	return &types.AWSResource{
		ID:       aws.ToString(instance.InstanceId),
		Type:     "instance",
		Region:   c.cfg.Region,
		State:    string(instance.State.Name),
		Tags:     tags,
		Details:  details,
		LastSeen: time.Now(),
	}
}

// findDefaultSubnet finds a default subnet in the default VPC with enhanced fallback logic
func (c *Client) findDefaultSubnet(ctx context.Context) (string, error) {
	c.logger.WithField("region", c.cfg.Region).Info("Starting search for default subnet")

	// First, find the default VPC
	vpcResult, err := c.ec2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("isDefault"),
				Values: []string{"true"},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe VPCs: %w", err)
	}

	var vpcID string
	if len(vpcResult.Vpcs) > 0 {
		vpcID = *vpcResult.Vpcs[0].VpcId
		c.logger.WithField("defaultVpcId", vpcID).Info("Found default VPC")
	} else {
		c.logger.Warn("No default VPC found, looking for any available VPC")

		// Fallback: find any available VPC
		fallbackVpcResult, err := c.ec2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
			Filters: []ec2types.Filter{
				{
					Name:   aws.String("state"),
					Values: []string{"available"},
				},
			},
		})
		if err != nil {
			return "", fmt.Errorf("failed to describe VPCs for fallback: %w", err)
		}

		if len(fallbackVpcResult.Vpcs) == 0 {
			return "", fmt.Errorf("no VPCs found in region %s", c.cfg.Region)
		}

		vpcID = *fallbackVpcResult.Vpcs[0].VpcId
		c.logger.WithField("fallbackVpcId", vpcID).Info("Using first available VPC as fallback")
	}

	// Find a subnet in the VPC - try default subnets first
	subnetResult, err := c.ec2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
			{
				Name:   aws.String("default-for-az"),
				Values: []string{"true"},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe subnets: %w", err)
	}

	// If we found default subnets, use the first available one
	if len(subnetResult.Subnets) > 0 {
		for _, subnet := range subnetResult.Subnets {
			if subnet.State == ec2types.SubnetStateAvailable {
				subnetID := *subnet.SubnetId
				c.logger.WithFields(logrus.Fields{
					"subnetId":  subnetID,
					"vpcId":     vpcID,
					"isDefault": true,
				}).Info("Found default subnet")
				return subnetID, nil
			}
		}
	}

	// Fallback: find any available subnet in the VPC
	c.logger.Warn("No default subnets found, looking for any available subnet in VPC")

	allSubnetsResult, err := c.ec2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe all subnets: %w", err)
	}

	if len(allSubnetsResult.Subnets) == 0 {
		return "", fmt.Errorf("no available subnets found in VPC %s", vpcID)
	}

	// Return the first available subnet
	subnetID := *allSubnetsResult.Subnets[0].SubnetId
	c.logger.WithFields(logrus.Fields{
		"subnetId":  subnetID,
		"vpcId":     vpcID,
		"isDefault": false,
		"fallback":  true,
	}).Info("Using first available subnet as fallback")

	return subnetID, nil
}

// DescribeInstances lists EC2 instances
func (c *Client) DescribeInstances(ctx context.Context) ([]*types.AWSResource, error) {
	result, err := c.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instances: %w", err)
	}

	var resources []*types.AWSResource
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			resources = append(resources, c.convertEC2Instance(instance))
		}
	}

	return resources, nil
}

// ListEC2Instances is an alias for DescribeInstances for MCP compatibility
func (c *Client) ListEC2Instances(ctx context.Context) ([]*types.AWSResource, error) {
	return c.DescribeInstances(ctx)
}

// GetEC2Instance gets a specific EC2 instance by ID
func (c *Client) GetEC2Instance(ctx context.Context, instanceID string) (*types.AWSResource, error) {
	result, err := c.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("instance %s not found", instanceID)
	}

	return c.convertEC2Instance(result.Reservations[0].Instances[0]), nil
}

// CreateAMI creates an Amazon Machine Image from an EC2 instance
func (c *Client) CreateAMI(ctx context.Context, instanceID, name, description string) (*types.AWSResource, error) {
	input := &ec2.CreateImageInput{
		InstanceId:  aws.String(instanceID),
		Name:        aws.String(name),
		Description: aws.String(description),
		NoReboot:    aws.Bool(true), // Don't reboot the instance during AMI creation
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeImage,
				Tags: []ec2types.Tag{
					{Key: aws.String("Name"), Value: aws.String(name)},
					{Key: aws.String("Source"), Value: aws.String(instanceID)},
					{Key: aws.String("Environment"), Value: aws.String("production-ready")},
					{Key: aws.String("CreatedBy"), Value: aws.String("github.com/versus-control/ai-infrastructure-agent")},
				},
			},
		},
	}

	result, err := c.ec2.CreateImage(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create AMI from instance %s: %w", instanceID, err)
	}

	// Wait for AMI to be available (this can take several minutes)
	logrus.WithFields(logrus.Fields{
		"ami_id":      *result.ImageId,
		"instance_id": instanceID,
	}).Info("AMI creation initiated, waiting for completion...")

	// Create resource object
	resource := &types.AWSResource{
		ID:     *result.ImageId,
		Type:   "ami",
		Region: c.cfg.Region,
		State:  "pending",
		Tags:   make(map[string]string),
		Details: map[string]interface{}{
			"name":               name,
			"description":        description,
			"source_instance_id": instanceID,
		},
		LastSeen: time.Now(),
	}

	return resource, nil
}

// WaitForAMI waits for an AMI to become available
func (c *Client) WaitForAMI(ctx context.Context, amiID string) error {
	maxWaitTime := 30 * time.Minute
	pollInterval := 30 * time.Second

	ctxWithTimeout, cancel := context.WithTimeout(ctx, maxWaitTime)
	defer cancel()

	for {
		select {
		case <-ctxWithTimeout.Done():
			return fmt.Errorf("timeout waiting for AMI %s to become available", amiID)
		default:
			result, err := c.ec2.DescribeImages(ctx, &ec2.DescribeImagesInput{
				ImageIds: []string{amiID},
			})
			if err != nil {
				return fmt.Errorf("failed to describe AMI %s: %w", amiID, err)
			}

			if len(result.Images) == 0 {
				return fmt.Errorf("AMI %s not found", amiID)
			}

			state := result.Images[0].State
			logrus.WithFields(logrus.Fields{
				"ami_id": amiID,
				"state":  state,
			}).Info("AMI status check")

			switch state {
			case ec2types.ImageStateAvailable:
				return nil
			case ec2types.ImageStateFailed:
				return fmt.Errorf("AMI %s creation failed", amiID)
			case ec2types.ImageStatePending:
				time.Sleep(pollInterval)
			default:
				time.Sleep(pollInterval)
			}
		}
	}
}

// GetAvailabilityZones retrieves all available availability zones in the current region
func (c *Client) GetAvailabilityZones(ctx context.Context) ([]string, error) {
	c.logger.WithField("region", c.cfg.Region).Info("Starting DescribeAvailabilityZones API call")

	result, err := c.ec2.DescribeAvailabilityZones(ctx, &ec2.DescribeAvailabilityZonesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	})
	if err != nil {
		c.logger.WithError(err).Error("DescribeAvailabilityZones API call failed")
		return nil, fmt.Errorf("failed to describe availability zones: %w", err)
	}

	c.logger.WithField("azs_count", len(result.AvailabilityZones)).Info("DescribeAvailabilityZones API call successful")

	var zones []string
	for _, az := range result.AvailabilityZones {
		if az.ZoneName != nil {
			zones = append(zones, *az.ZoneName)
			c.logger.WithFields(logrus.Fields{
				"az_name": *az.ZoneName,
				"state":   az.State,
			}).Debug("Found available AZ")
		}
	}

	if len(zones) == 0 {
		// Fallback to common zones for the current region if none found
		c.logger.Warn("No availability zones found, using fallback zones")
		fallbackZones := []string{c.cfg.Region + "a", c.cfg.Region + "b", c.cfg.Region + "c"}
		return fallbackZones, nil
	}

	c.logger.WithFields(logrus.Fields{
		"availability_zones": zones,
		"region":             c.cfg.Region,
		"count":              len(zones),
	}).Info("Successfully found availability zones via AWS API")

	return zones, nil
}

// ========== AMI Listing Methods ==========

// DescribeAMIs lists all AMIs owned by the account
func (c *Client) DescribeAMIs(ctx context.Context) ([]*types.AWSResource, error) {
	result, err := c.ec2.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"self"}, // Only show AMIs owned by this account
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe AMIs: %w", err)
	}

	var resources []*types.AWSResource
	for _, image := range result.Images {
		resources = append(resources, c.convertAMI(image))
	}

	return resources, nil
}

// DescribePublicAMIs lists public AMIs with optional filters
func (c *Client) DescribePublicAMIs(ctx context.Context, namePattern string) ([]*types.AWSResource, error) {
	input := &ec2.DescribeImagesInput{
		Owners: []string{"amazon"}, // Amazon-owned public AMIs
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
			{
				Name:   aws.String("image-type"),
				Values: []string{"machine"},
			},
		},
	}

	if namePattern != "" {
		input.Filters = append(input.Filters, ec2types.Filter{
			Name:   aws.String("name"),
			Values: []string{namePattern},
		})
	}

	result, err := c.ec2.DescribeImages(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe public AMIs: %w", err)
	}

	var resources []*types.AWSResource
	for _, image := range result.Images {
		resources = append(resources, c.convertAMI(image))
	}

	return resources, nil
}

// GetAMI gets a specific AMI by ID
func (c *Client) GetAMI(ctx context.Context, amiID string) (*types.AWSResource, error) {
	result, err := c.ec2.DescribeImages(ctx, &ec2.DescribeImagesInput{
		ImageIds: []string{amiID},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe AMI %s: %w", amiID, err)
	}

	if len(result.Images) == 0 {
		return nil, fmt.Errorf("AMI %s not found", amiID)
	}

	return c.convertAMI(result.Images[0]), nil
}

// convertAMI converts an EC2 Image to our internal resource representation
func (c *Client) convertAMI(image ec2types.Image) *types.AWSResource {
	details := map[string]interface{}{
		"name":               aws.ToString(image.Name),
		"description":        aws.ToString(image.Description),
		"imageType":          string(image.ImageType),
		"kernelId":           aws.ToString(image.KernelId),
		"ramdiskId":          aws.ToString(image.RamdiskId),
		"platform":           string(image.Platform),
		"platformDetails":    aws.ToString(image.PlatformDetails),
		"usageOperation":     aws.ToString(image.UsageOperation),
		"architecture":       string(image.Architecture),
		"creationDate":       aws.ToString(image.CreationDate),
		"imageLocation":      aws.ToString(image.ImageLocation),
		"imageOwnerAlias":    aws.ToString(image.ImageOwnerAlias),
		"ownerId":            aws.ToString(image.OwnerId),
		"rootDeviceName":     aws.ToString(image.RootDeviceName),
		"rootDeviceType":     string(image.RootDeviceType),
		"sriovNetSupport":    aws.ToString(image.SriovNetSupport),
		"virtualizationType": string(image.VirtualizationType),
		"hypervisor":         string(image.Hypervisor),
		"public":             aws.ToBool(image.Public),
		"deprecationTime":    aws.ToString(image.DeprecationTime),
	}

	// Add block device mappings
	if len(image.BlockDeviceMappings) > 0 {
		var mappings []map[string]interface{}
		for _, bdm := range image.BlockDeviceMappings {
			mapping := map[string]interface{}{
				"deviceName":  aws.ToString(bdm.DeviceName),
				"virtualName": aws.ToString(bdm.VirtualName),
			}
			if bdm.Ebs != nil {
				mapping["ebs"] = map[string]interface{}{
					"volumeSize":          aws.ToInt32(bdm.Ebs.VolumeSize),
					"volumeType":          string(bdm.Ebs.VolumeType),
					"deleteOnTermination": aws.ToBool(bdm.Ebs.DeleteOnTermination),
					"encrypted":           aws.ToBool(bdm.Ebs.Encrypted),
					"snapshotId":          aws.ToString(bdm.Ebs.SnapshotId),
					"kmsKeyId":            aws.ToString(bdm.Ebs.KmsKeyId),
					"iops":                aws.ToInt32(bdm.Ebs.Iops),
					"throughput":          aws.ToInt32(bdm.Ebs.Throughput),
				}
			}
			mappings = append(mappings, mapping)
		}
		details["blockDeviceMappings"] = mappings
	}

	return &types.AWSResource{
		ID:       aws.ToString(image.ImageId),
		Type:     "ami",
		Region:   c.cfg.Region,
		State:    string(image.State),
		Tags:     make(map[string]string), // Tags need to be fetched separately or converted from image.Tags
		Details:  details,
		LastSeen: time.Now(),
	}
}

// GetLatestAmazonLinux2AMI finds the latest Amazon Linux 2 AMI in the current region
func (c *Client) GetLatestAmazonLinux2AMI(ctx context.Context) (string, error) {
	c.logger.WithField("region", c.cfg.Region).Info("Starting DescribeImages API call for latest Amazon Linux 2 AMI")

	input := &ec2.DescribeImagesInput{
		Owners: []string{"amazon"},
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{"amzn2-ami-hvm-*-x86_64-gp2"},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	}

	c.logger.WithFields(logrus.Fields{
		"owners":  input.Owners,
		"filters": input.Filters,
	}).Debug("Making DescribeImages API call with parameters")

	result, err := c.ec2.DescribeImages(ctx, input)
	if err != nil {
		c.logger.WithError(err).Error("DescribeImages API call failed")
		return "", fmt.Errorf("failed to describe AMIs: %w", err)
	}

	c.logger.WithField("images_count", len(result.Images)).Info("DescribeImages API call successful")

	if len(result.Images) == 0 {
		c.logger.Error("No Amazon Linux 2 AMIs found in current region")
		return "", fmt.Errorf("no Amazon Linux 2 AMIs found")
	}

	// Find the most recent AMI by creation date
	var latestAMI ec2types.Image
	var latestTime time.Time

	c.logger.Debug("Processing AMI images to find latest")
	for i, image := range result.Images {
		if image.CreationDate == nil {
			c.logger.WithField("ami_index", i).Warn("AMI has no creation date, skipping")
			continue
		}

		creationTime, err := time.Parse(time.RFC3339, *image.CreationDate)
		if err != nil {
			c.logger.WithError(err).WithField("ami", *image.ImageId).Warn("Failed to parse AMI creation date")
			continue
		}

		c.logger.WithFields(logrus.Fields{
			"ami_id":        *image.ImageId,
			"name":          aws.ToString(image.Name),
			"creation_date": *image.CreationDate,
		}).Debug("Processing AMI candidate")

		if creationTime.After(latestTime) {
			latestTime = creationTime
			latestAMI = image
			c.logger.WithField("new_latest_ami", *image.ImageId).Debug("Found newer AMI")
		}
	}

	if latestAMI.ImageId == nil {
		c.logger.Error("No valid Amazon Linux 2 AMI found after processing all images")
		return "", fmt.Errorf("no valid Amazon Linux 2 AMI found")
	}

	c.logger.WithFields(logrus.Fields{
		"amiId":        *latestAMI.ImageId,
		"name":         aws.ToString(latestAMI.Name),
		"creationDate": aws.ToString(latestAMI.CreationDate),
		"region":       c.cfg.Region,
	}).Info("Successfully found latest Amazon Linux 2 AMI via AWS API")

	return *latestAMI.ImageId, nil
}

// GetLatestUbuntuAMI finds the latest Ubuntu LTS AMI in the current region
func (c *Client) GetLatestUbuntuAMI(ctx context.Context, architecture string) (string, error) {
	c.logger.WithFields(logrus.Fields{
		"region":       c.cfg.Region,
		"architecture": architecture,
	}).Info("Starting DescribeImages API call for latest Ubuntu LTS AMI")

	// Ubuntu AMI name pattern for LTS versions
	namePattern := "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-" + architecture + "-server-*"
	if architecture == "arm64" {
		namePattern = "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-arm64-server-*"
	}

	input := &ec2.DescribeImagesInput{
		Owners: []string{"099720109477"}, // Canonical's AWS account ID
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{namePattern},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	}

	result, err := c.ec2.DescribeImages(ctx, input)
	if err != nil {
		c.logger.WithError(err).Error("DescribeImages API call failed for Ubuntu AMI")
		return "", fmt.Errorf("failed to describe Ubuntu AMIs: %w", err)
	}

	c.logger.WithField("images_count", len(result.Images)).Info("DescribeImages API call successful for Ubuntu")

	if len(result.Images) == 0 {
		c.logger.Error("No Ubuntu AMIs found in current region")
		return "", fmt.Errorf("no Ubuntu AMIs found")
	}

	// Find the most recent AMI by creation date
	var latestAMI ec2types.Image
	var latestTime time.Time

	for _, image := range result.Images {
		if image.CreationDate == nil {
			continue
		}

		creationTime, err := time.Parse(time.RFC3339, *image.CreationDate)
		if err != nil {
			c.logger.WithError(err).WithField("ami", *image.ImageId).Warn("Failed to parse Ubuntu AMI creation date")
			continue
		}

		if creationTime.After(latestTime) {
			latestTime = creationTime
			latestAMI = image
		}
	}

	if latestAMI.ImageId == nil {
		return "", fmt.Errorf("no valid Ubuntu AMI found")
	}

	c.logger.WithFields(logrus.Fields{
		"amiId":        *latestAMI.ImageId,
		"name":         aws.ToString(latestAMI.Name),
		"creationDate": aws.ToString(latestAMI.CreationDate),
		"region":       c.cfg.Region,
	}).Info("Successfully found latest Ubuntu LTS AMI via AWS API")

	return *latestAMI.ImageId, nil
}

// GetLatestWindowsAMI finds the latest Windows Server AMI in the current region
func (c *Client) GetLatestWindowsAMI(ctx context.Context, architecture string) (string, error) {
	c.logger.WithFields(logrus.Fields{
		"region":       c.cfg.Region,
		"architecture": architecture,
	}).Info("Starting DescribeImages API call for latest Windows Server AMI")

	// Windows Server 2022 Base AMI pattern
	namePattern := "Windows_Server-2022-English-Full-Base-*"

	input := &ec2.DescribeImagesInput{
		Owners: []string{"amazon"},
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{namePattern},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
			{
				Name:   aws.String("architecture"),
				Values: []string{architecture},
			},
		},
	}

	result, err := c.ec2.DescribeImages(ctx, input)
	if err != nil {
		c.logger.WithError(err).Error("DescribeImages API call failed for Windows AMI")
		return "", fmt.Errorf("failed to describe Windows AMIs: %w", err)
	}

	c.logger.WithField("images_count", len(result.Images)).Info("DescribeImages API call successful for Windows")

	if len(result.Images) == 0 {
		c.logger.Error("No Windows Server AMIs found in current region")
		return "", fmt.Errorf("no Windows Server AMIs found")
	}

	// Find the most recent AMI by creation date
	var latestAMI ec2types.Image
	var latestTime time.Time

	for _, image := range result.Images {
		if image.CreationDate == nil {
			continue
		}

		creationTime, err := time.Parse(time.RFC3339, *image.CreationDate)
		if err != nil {
			c.logger.WithError(err).WithField("ami", *image.ImageId).Warn("Failed to parse Windows AMI creation date")
			continue
		}

		if creationTime.After(latestTime) {
			latestTime = creationTime
			latestAMI = image
		}
	}

	if latestAMI.ImageId == nil {
		return "", fmt.Errorf("no valid Windows Server AMI found")
	}

	c.logger.WithFields(logrus.Fields{
		"amiId":        *latestAMI.ImageId,
		"name":         aws.ToString(latestAMI.Name),
		"creationDate": aws.ToString(latestAMI.CreationDate),
		"region":       c.cfg.Region,
	}).Info("Successfully found latest Windows Server AMI via AWS API")

	return *latestAMI.ImageId, nil
}

// GetDefaultVPC finds the default VPC in the current region, with fallback to first available VPC
func (c *Client) GetDefaultVPC(ctx context.Context) (string, error) {
	c.logger.WithField("region", c.cfg.Region).Info("Starting DescribeVpcs API call for default VPC")

	// First try to find the default VPC
	input := &ec2.DescribeVpcsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("isDefault"),
				Values: []string{"true"},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	}

	result, err := c.ec2.DescribeVpcs(ctx, input)
	if err != nil {
		c.logger.WithError(err).Error("DescribeVpcs API call failed")
		return "", fmt.Errorf("failed to describe VPCs: %w", err)
	}

	c.logger.WithField("default_vpcs_count", len(result.Vpcs)).Info("DescribeVpcs API call successful for default VPC")

	// If we found a default VPC, use it
	if len(result.Vpcs) > 0 {
		defaultVpcID := *result.Vpcs[0].VpcId
		c.logger.WithFields(logrus.Fields{
			"vpcId":     defaultVpcID,
			"region":    c.cfg.Region,
			"isDefault": true,
		}).Info("Successfully found default VPC via AWS API")
		return defaultVpcID, nil
	}

	// If no default VPC found, try to find any available VPC as fallback
	c.logger.Warn("No default VPC found, searching for any available VPC as fallback")

	fallbackInput := &ec2.DescribeVpcsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	}

	fallbackResult, err := c.ec2.DescribeVpcs(ctx, fallbackInput)
	if err != nil {
		c.logger.WithError(err).Error("Fallback DescribeVpcs API call failed")
		return "", fmt.Errorf("failed to describe VPCs for fallback: %w", err)
	}

	c.logger.WithField("available_vpcs_count", len(fallbackResult.Vpcs)).Info("Fallback DescribeVpcs API call successful")

	if len(fallbackResult.Vpcs) == 0 {
		c.logger.Error("No VPCs found in current region")
		return "", fmt.Errorf("no VPCs found in region %s", c.cfg.Region)
	}

	// Use the first available VPC
	fallbackVpcID := *fallbackResult.Vpcs[0].VpcId
	c.logger.WithFields(logrus.Fields{
		"vpcId":     fallbackVpcID,
		"region":    c.cfg.Region,
		"isDefault": false,
		"fallback":  true,
	}).Info("Using first available VPC as fallback")

	return fallbackVpcID, nil
}

// GetDefaultSubnet finds a default subnet, with fallback logic similar to findDefaultSubnet
func (c *Client) GetDefaultSubnet(ctx context.Context) (*SubnetInfo, error) {
	c.logger.WithField("region", c.cfg.Region).Info("Starting API call to get default subnet")

	// Use the existing findDefaultSubnet logic which already has robust fallback
	subnetID, err := c.findDefaultSubnet(ctx)
	if err != nil {
		c.logger.WithError(err).Error("Failed to find default subnet via AWS API")
		return nil, fmt.Errorf("failed to find default subnet: %w", err)
	}

	// Get subnet details to extract VPC ID
	subnetResult, err := c.ec2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		SubnetIds: []string{subnetID},
	})
	if err != nil {
		c.logger.WithError(err).Error("Failed to describe subnet for VPC ID")
		return nil, fmt.Errorf("failed to describe subnet: %w", err)
	}

	if len(subnetResult.Subnets) == 0 {
		return nil, fmt.Errorf("subnet %s not found", subnetID)
	}

	vpcID := aws.ToString(subnetResult.Subnets[0].VpcId)

	c.logger.WithFields(logrus.Fields{
		"subnetId": subnetID,
		"vpcId":    vpcID,
		"region":   c.cfg.Region,
	}).Info("Successfully found default subnet and VPC via AWS API")

	return &SubnetInfo{
		SubnetID: subnetID,
		VpcID:    vpcID,
	}, nil
}

// GetSubnetsInVPC gets all available subnets in a specific VPC
func (c *Client) GetSubnetsInVPC(ctx context.Context, vpcID string) ([]string, error) {
	c.logger.WithFields(logrus.Fields{
		"vpcId":  vpcID,
		"region": c.cfg.Region,
	}).Info("Starting DescribeSubnets API call for VPC")

	input := &ec2.DescribeSubnetsInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	}

	result, err := c.ec2.DescribeSubnets(ctx, input)
	if err != nil {
		c.logger.WithError(err).Error("DescribeSubnets API call failed")
		return nil, fmt.Errorf("failed to describe subnets: %w", err)
	}

	c.logger.WithField("subnets_count", len(result.Subnets)).Info("DescribeSubnets API call successful")

	if len(result.Subnets) == 0 {
		c.logger.Warn("No subnets found in VPC")
		return []string{}, nil
	}

	var subnetIDs []string
	for _, subnet := range result.Subnets {
		if subnet.SubnetId != nil {
			subnetIDs = append(subnetIDs, *subnet.SubnetId)
			c.logger.WithFields(logrus.Fields{
				"subnetId": *subnet.SubnetId,
				"az":       aws.ToString(subnet.AvailabilityZone),
				"cidr":     aws.ToString(subnet.CidrBlock),
			}).Debug("Found subnet in VPC")
		}
	}

	c.logger.WithFields(logrus.Fields{
		"subnet_ids": subnetIDs,
		"vpcId":      vpcID,
		"region":     c.cfg.Region,
		"count":      len(subnetIDs),
	}).Info("Successfully found subnets in VPC via AWS API")

	return subnetIDs, nil
}

// ListAMIs lists Amazon Machine Images owned by the specified owner
func (c *Client) ListAMIs(ctx context.Context, owner string) ([]*types.AWSResource, error) {
	input := &ec2.DescribeImagesInput{
		Owners: []string{owner},
	}

	result, err := c.ec2.DescribeImages(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list AMIs for owner %s: %w", owner, err)
	}

	var amis []*types.AWSResource
	for _, image := range result.Images {
		if image.ImageId == nil {
			continue
		}

		// Convert tags
		tags := make(map[string]string)
		for _, tag := range image.Tags {
			if tag.Key != nil && tag.Value != nil {
				tags[*tag.Key] = *tag.Value
			}
		}

		ami := &types.AWSResource{
			ID:     *image.ImageId,
			Type:   "ami",
			Region: c.cfg.Region,
			State:  string(image.State),
			Tags:   tags,
			Details: map[string]interface{}{
				"name":               aws.ToString(image.Name),
				"description":        aws.ToString(image.Description),
				"architecture":       string(image.Architecture),
				"platform":           string(image.Platform),
				"virtualizationType": string(image.VirtualizationType),
				"rootDeviceType":     string(image.RootDeviceType),
				"creationDate":       aws.ToString(image.CreationDate),
				"public":             aws.ToBool(image.Public),
			},
		}

		amis = append(amis, ami)
	}

	c.logger.WithFields(logrus.Fields{
		"owner":  owner,
		"count":  len(amis),
		"region": c.cfg.Region,
	}).Info("Successfully listed AMIs via AWS API")

	return amis, nil
}
