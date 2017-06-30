package core_test

import (
	//	"fmt"
	"github.com/APTrust/bagit/core"
	"github.com/APTrust/bagit/util/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func getValidator(t *testing.T, bagName, profileName string) *core.Validator {
	profilePath, err := testutil.GetPathToTestProfile(profileName)
	require.Nil(t, err)
	profile, err := core.LoadBagItProfile(profilePath)
	require.Nil(t, err)

	bagPath, err := testutil.GetPathToTestBag(bagName)
	require.Nil(t, err)
	bag := core.NewBag(bagPath)
	return core.NewValidator(bag, profile)
}

func TestNewValidator(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.0.json")
	assert.NotNil(t, validator)
}

func TestReadBag(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.0.json")
	require.NotNil(t, validator)
	validator.ReadBag()
	assert.Empty(t, validator.Errors())
	assert.Equal(t, 4, len(validator.Bag.Payload))
	assert.Equal(t, 2, len(validator.Bag.Manifests))
	assert.Equal(t, 2, len(validator.Bag.TagManifests))
	assert.Equal(t, 8, len(validator.Bag.TagFiles))

	assert.NotEmpty(t, validator.Bag.Payload["data/datastream-DC"])
	assert.NotEmpty(t, validator.Bag.Payload["data/datastream-descMetadata"])
	assert.NotEmpty(t, validator.Bag.Payload["data/datastream-MARC"])
	assert.NotEmpty(t, validator.Bag.Payload["data/datastream-RELS-EXT"])

	assert.NotEmpty(t, validator.Bag.Manifests["manifest-md5.txt"])
	assert.NotEmpty(t, validator.Bag.Manifests["manifest-sha256.txt"])

	assert.NotEmpty(t, validator.Bag.TagManifests["tagmanifest-md5.txt"])
	assert.NotEmpty(t, validator.Bag.TagManifests["tagmanifest-sha256.txt"])

	assert.NotEmpty(t, validator.Bag.TagFiles["aptrust-info.txt"])
	assert.NotEmpty(t, validator.Bag.TagFiles["bag-info.txt"])
	assert.NotEmpty(t, validator.Bag.TagFiles["bagit.txt"])
	assert.NotEmpty(t, validator.Bag.TagFiles["custom_tag_file.txt"])
	assert.NotEmpty(t, validator.Bag.TagFiles["junk_file.txt"])
	assert.NotEmpty(t, validator.Bag.TagFiles["custom_tags/tracked_file_custom.xml"])
	assert.NotEmpty(t, validator.Bag.TagFiles["custom_tags/tracked_tag_file.txt"])
	assert.NotEmpty(t, validator.Bag.TagFiles["custom_tags/untracked_tag_file.txt"])
}
