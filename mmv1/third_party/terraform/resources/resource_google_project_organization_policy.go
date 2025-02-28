package google

import (
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/api/cloudresourcemanager/v1"
)

func resourceGoogleProjectOrganizationPolicy() *schema.Resource {
	return &schema.Resource{
		Create: resourceGoogleProjectOrganizationPolicyCreate,
		Read:   resourceGoogleProjectOrganizationPolicyRead,
		Update: resourceGoogleProjectOrganizationPolicyUpdate,
		Delete: resourceGoogleProjectOrganizationPolicyDelete,

		Importer: &schema.ResourceImporter{
			State: resourceProjectOrgPolicyImporter,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(4 * time.Minute),
			Update: schema.DefaultTimeout(4 * time.Minute),
			Read:   schema.DefaultTimeout(4 * time.Minute),
			Delete: schema.DefaultTimeout(4 * time.Minute),
		},

		Schema: mergeSchemas(
			schemaOrganizationPolicy,
			map[string]*schema.Schema{
				"project": {
					Type:        schema.TypeString,
					Required:    true,
					ForceNew:    true,
					Description: `The project ID.`,
				},
			},
		),
		UseJSONNumber: true,
	}
}

func resourceProjectOrgPolicyImporter(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	config := meta.(*Config)

	if err := parseImportId([]string{
		"projects/(?P<project>[^/]+):constraints/(?P<constraint>[^/]+)",
		"(?P<project>[^/]+):constraints/(?P<constraint>[^/]+)",
		"(?P<project>[^/]+):(?P<constraint>[^/]+)"},
		d, config); err != nil {
		return nil, err
	}

	if d.Get("project") == "" || d.Get("constraint") == "" {
		return nil, fmt.Errorf("unable to parse project or constraint. Check import formats")
	}

	return []*schema.ResourceData{d}, nil
}

func resourceGoogleProjectOrganizationPolicyCreate(d *schema.ResourceData, meta interface{}) error {
	if isOrganizationPolicyUnset(d) {
		d.SetId(fmt.Sprintf("%s:%s", d.Get("project"), d.Get("constraint")))
		return resourceGoogleProjectOrganizationPolicyDelete(d, meta)
	}

	if err := setProjectOrganizationPolicy(d, meta); err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("%s:%s", d.Get("project"), d.Get("constraint")))

	return resourceGoogleProjectOrganizationPolicyRead(d, meta)
}

func resourceGoogleProjectOrganizationPolicyRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	userAgent, err := generateUserAgentString(d, config.userAgent)
	if err != nil {
		return err
	}
	project := prefixedProject(d.Get("project").(string))

	var policy *cloudresourcemanager.OrgPolicy
	err = retryTimeDuration(func() (readErr error) {
		policy, readErr = config.NewResourceManagerClient(userAgent).Projects.GetOrgPolicy(project, &cloudresourcemanager.GetOrgPolicyRequest{
			Constraint: canonicalOrgPolicyConstraint(d.Get("constraint").(string)),
		}).Do()
		return readErr
	}, d.Timeout(schema.TimeoutRead))
	if err != nil {
		return handleNotFoundError(err, d, fmt.Sprintf("Organization policy for %s", project))
	}

	if err := d.Set("constraint", policy.Constraint); err != nil {
		return fmt.Errorf("Error setting constraint: %s", err)
	}
	if err := d.Set("boolean_policy", flattenBooleanOrganizationPolicy(policy.BooleanPolicy)); err != nil {
		return fmt.Errorf("Error setting boolean_policy: %s", err)
	}
	if err := d.Set("list_policy", flattenListOrganizationPolicy(policy.ListPolicy)); err != nil {
		return fmt.Errorf("Error setting list_policy: %s", err)
	}
	if err := d.Set("restore_policy", flattenRestoreOrganizationPolicy(policy.RestoreDefault)); err != nil {
		return fmt.Errorf("Error setting restore_policy: %s", err)
	}
	if err := d.Set("version", policy.Version); err != nil {
		return fmt.Errorf("Error setting version: %s", err)
	}
	if err := d.Set("etag", policy.Etag); err != nil {
		return fmt.Errorf("Error setting etag: %s", err)
	}
	if err := d.Set("update_time", policy.UpdateTime); err != nil {
		return fmt.Errorf("Error setting update_time: %s", err)
	}

	return nil
}

func resourceGoogleProjectOrganizationPolicyUpdate(d *schema.ResourceData, meta interface{}) error {
	if isOrganizationPolicyUnset(d) {
		return resourceGoogleProjectOrganizationPolicyDelete(d, meta)
	}

	if err := setProjectOrganizationPolicy(d, meta); err != nil {
		return err
	}

	return resourceGoogleProjectOrganizationPolicyRead(d, meta)
}

func resourceGoogleProjectOrganizationPolicyDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	userAgent, err := generateUserAgentString(d, config.userAgent)
	if err != nil {
		return err
	}
	project := prefixedProject(d.Get("project").(string))

	return retryTimeDuration(func() error {
		_, err := config.NewResourceManagerClient(userAgent).Projects.ClearOrgPolicy(project, &cloudresourcemanager.ClearOrgPolicyRequest{
			Constraint: canonicalOrgPolicyConstraint(d.Get("constraint").(string)),
		}).Do()
		return err
	}, d.Timeout(schema.TimeoutDelete))
}

func setProjectOrganizationPolicy(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)
	userAgent, err := generateUserAgentString(d, config.userAgent)
	if err != nil {
		return err
	}

	project := prefixedProject(d.Get("project").(string))

	listPolicy, err := expandListOrganizationPolicy(d.Get("list_policy").([]interface{}))
	if err != nil {
		return err
	}

	restore_default, err := expandRestoreOrganizationPolicy(d.Get("restore_policy").([]interface{}))
	if err != nil {
		return err
	}

	return retryTimeDuration(func() error {
		_, err := config.NewResourceManagerClient(userAgent).Projects.SetOrgPolicy(project, &cloudresourcemanager.SetOrgPolicyRequest{
			Policy: &cloudresourcemanager.OrgPolicy{
				Constraint:     canonicalOrgPolicyConstraint(d.Get("constraint").(string)),
				BooleanPolicy:  expandBooleanOrganizationPolicy(d.Get("boolean_policy").([]interface{})),
				ListPolicy:     listPolicy,
				RestoreDefault: restore_default,
				Version:        int64(d.Get("version").(int)),
				Etag:           d.Get("etag").(string),
			},
		}).Do()
		return err
	}, d.Timeout(schema.TimeoutCreate))
}
