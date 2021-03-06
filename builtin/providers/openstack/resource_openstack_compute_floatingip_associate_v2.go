package openstack

import (
	"fmt"
	"log"
	"strings"

	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/floatingips"
	"github.com/hashicorp/terraform/helper/schema"
)

func resourceComputeFloatingIPAssociateV2() *schema.Resource {
	return &schema.Resource{
		Create: resourceComputeFloatingIPAssociateV2Create,
		Read:   resourceComputeFloatingIPAssociateV2Read,
		Delete: resourceComputeFloatingIPAssociateV2Delete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"region": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				DefaultFunc: schema.EnvDefaultFunc("OS_REGION_NAME", ""),
			},
			"floating_ip": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"instance_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"fixed_ip": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

func resourceComputeFloatingIPAssociateV2Create(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	computeClient, err := config.computeV2Client(GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenStack compute client: %s", err)
	}

	floatingIP := d.Get("floating_ip").(string)
	fixedIP := d.Get("fixed_ip").(string)
	instanceId := d.Get("instance_id").(string)

	associateOpts := floatingips.AssociateOpts{
		FloatingIP: floatingIP,
		FixedIP:    fixedIP,
	}
	log.Printf("[DEBUG] Associate Options: %#v", associateOpts)

	err = floatingips.AssociateInstance(computeClient, instanceId, associateOpts).ExtractErr()
	if err != nil {
		return fmt.Errorf("Error associating Floating IP: %s", err)
	}

	// There's an API call to get this information, but it has been
	// deprecated. The Neutron API could be used, but I'm trying not
	// to mix service APIs. Therefore, a faux ID will be used.
	id := fmt.Sprintf("%s/%s/%s", floatingIP, instanceId, fixedIP)
	d.SetId(id)

	// This API call is synchronous, so Create won't return until the IP
	// is attached. No need to wait for a state.

	return resourceComputeFloatingIPAssociateV2Read(d, meta)
}

func resourceComputeFloatingIPAssociateV2Read(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	computeClient, err := config.computeV2Client(GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenStack compute client: %s", err)
	}

	// Obtain relevant info from parsing the ID
	floatingIP, instanceId, fixedIP, err := parseComputeFloatingIPAssociateId(d.Id())
	if err != nil {
		return err
	}

	d.Set("floating_ip", floatingIP)
	d.Set("instance_id", instanceId)
	d.Set("fixed_ip", fixedIP)
	d.Set("region", GetRegion(d))

	// check whether floatingIP or associated instance deleted
	fip, err := floatingips.Get(computeClient, floatingIP).Extract()
	if err != nil {
		if CheckDeleted(d, err, "floating ip") == nil {
			return nil
		}
	}
	if fip.InstanceID == "" {
		d.SetId("")
	}

	return nil
}

func resourceComputeFloatingIPAssociateV2Delete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	computeClient, err := config.computeV2Client(GetRegion(d))
	if err != nil {
		return fmt.Errorf("Error creating OpenStack compute client: %s", err)
	}

	floatingIP := d.Get("floating_ip").(string)
	instanceId := d.Get("instance_id").(string)

	disassociateOpts := floatingips.DisassociateOpts{
		FloatingIP: floatingIP,
	}
	log.Printf("[DEBUG] Disssociate Options: %#v", disassociateOpts)

	err = floatingips.DisassociateInstance(computeClient, instanceId, disassociateOpts).ExtractErr()
	if err != nil {
		return CheckDeleted(d, err, "floating ip association")
	}

	return nil
}

func parseComputeFloatingIPAssociateId(id string) (string, string, string, error) {
	idParts := strings.Split(id, "/")
	if len(idParts) < 3 {
		return "", "", "", fmt.Errorf("Unable to determine floating ip association ID")
	}

	floatingIP := idParts[0]
	instanceId := idParts[1]
	fixedIP := idParts[2]

	return floatingIP, instanceId, fixedIP, nil
}
