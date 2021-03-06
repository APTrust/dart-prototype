package bagit_test

import (
	"github.com/APTrust/dart/bagit"
	"github.com/APTrust/dart/constants"
	"github.com/APTrust/dart/util"
	"github.com/APTrust/dart/util/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
)

func getValidator(t *testing.T, bagName, profileName string) *bagit.Validator {
	profilePath, err := testutil.GetPathToTestProfile(profileName)
	require.Nil(t, err)
	profile, err := bagit.LoadBagItProfile(profilePath)
	require.Nil(t, err)

	bagPath, err := testutil.GetPathToTestBag(bagName)
	require.Nil(t, err)
	bag := bagit.NewBag(bagPath)
	return bagit.NewValidator(bag, profile)
}

func TestNewValidator(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	assert.NotNil(t, validator)
}

func TestReadBag(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	validator.ReadBag()
	assert.Empty(t, validator.Errors())

	// Make sure files were added as the right types, in the right places.
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

	// Manifests should have been parsed.
	for _, m := range validator.Bag.Manifests {
		assert.Equal(t, 4, m.ParsedData.Count())
	}
	md5, err := validator.Bag.GetChecksumFromManifest(constants.MD5, "data/datastream-DC")
	require.Nil(t, err)
	sha256, err := validator.Bag.GetChecksumFromManifest(constants.SHA256, "data/datastream-DC")
	require.Nil(t, err)
	assert.Equal(t, "44d85cf4810d6c6fe87750117633e461", md5)
	assert.Equal(t, "248fac506a5c46b3c760312b99827b6fb5df4698d6cf9a9cdc4c54746728ab99", sha256)

	// TagManifests should have been parsed.
	for _, m := range validator.Bag.TagManifests {
		assert.Equal(t, 8, m.ParsedData.Count())
	}
	md5, err = validator.Bag.GetChecksumFromTagManifest(constants.MD5, "aptrust-info.txt")
	require.Nil(t, err)
	sha256, err = validator.Bag.GetChecksumFromTagManifest(constants.SHA256, "aptrust-info.txt")
	require.Nil(t, err)
	assert.Equal(t, "300e936e622605f9f7a846d261d53093", md5)
	assert.Equal(t, "a2b6c5a713af771c5e4edde8d5be25fbcad86e45ea338f43a5bb769347e7c8bb", sha256)

	// Files in BagItProfile.TagFilesRequired should be parsed,
	// while others should not.
	unparsedTagFile := validator.Bag.TagFiles["custom_tags/tracked_tag_file.txt"]
	assert.Equal(t, 0, unparsedTagFile.ParsedData.Count())

	parsedTagFile := validator.Bag.TagFiles["aptrust-info.txt"]
	require.NotEqual(t, 0, parsedTagFile.ParsedData.Count())
	require.Equal(t, 1, len(parsedTagFile.ParsedData.FindByKey("Title")))
	require.Equal(t, 1, len(parsedTagFile.ParsedData.FindByKey("Access")))
	assert.Equal(t, "Thirteen Ways of Looking at a Blackbird", parsedTagFile.ParsedData.FindByKey("Title")[0].Value)
	assert.Equal(t, "Institution", parsedTagFile.ParsedData.FindByKey("Access")[0].Value)
}

func TestValidateTopLevelFiles(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)

	validator.Profile.AllowMiscTopLevelFiles = true
	validator.ReadBag()
	assert.True(t, validator.ValidateTopLevelFiles())
	assert.Empty(t, validator.Errors())

	validator = getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)

	// Items in the top-level dir that are not manifests or required
	// tag files are considered misc. The APTrust BagIt profile
	// defines 3 of the 5 non-manifest items in the top-level dir
	// as required (bagit, bag-info, aptrust-info), so the other two
	// are misc.
	validator.Profile.AllowMiscTopLevelFiles = false
	validator.ReadBag()
	assert.False(t, validator.ValidateTopLevelFiles())
	errs := validator.Errors()
	require.Equal(t, 2, len(errs), "%v", errs)
	// These two may come back in different order.
	assert.True(t, strings.Contains(errs[0], "custom_tag_file.txt") ||
		strings.Contains(errs[0], "junk_file.txt"))
	assert.True(t, strings.Contains(errs[1], "custom_tag_file.txt") ||
		strings.Contains(errs[1], "junk_file.txt"))
}

func TestValidateMiscDirectories(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	validator.Profile.AllowMiscDirectories = true
	validator.ReadBag()
	assert.True(t, validator.ValidateMiscDirectories())
	assert.Empty(t, validator.Errors())

	validator = getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	validator.Profile.AllowMiscDirectories = false
	validator.ReadBag()
	assert.False(t, validator.ValidateMiscDirectories())
	require.NotEmpty(t, validator.Errors())
	assert.Equal(t, "Directory 'custom_tags' is not allowed in top-level directory when BagIt profile says AllowMiscDirectories is false.", validator.Errors()[0])
}

func TestValidateBagItVersion(t *testing.T) {
	// Both profile and bag say version 0.97
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	validator.ReadBag()
	assert.True(t, validator.ValidateBagItVersion())
	assert.Empty(t, validator.Errors())

	// If no accepted versions are specified, then any version will do.
	validator = getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	validator.Profile.AcceptBagItVersion = nil
	validator.ReadBag()
	assert.True(t, validator.ValidateBagItVersion())
	assert.Empty(t, validator.Errors())

	// Mismatch between accepted versions and actual version should
	// cause a validation error.
	validator = getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	validator.Profile.AcceptBagItVersion = []string{"2.22", "3.33", "4.44"}
	validator.ReadBag()
	assert.False(t, validator.ValidateBagItVersion())
	require.Equal(t, 1, len(validator.Errors()))
	assert.Equal(t, "BagIt version 0.97 in bagit.txt does not match allowed version(s) 2.22,3.33,4.44", validator.Errors()[0])

	// Be specific about missing BagIt version
	validator = getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	validator.ReadBag()
	validator.Bag.TagFiles["bagit.txt"].ParsedData.DeleteByKey("BagIt-Version")
	assert.False(t, validator.ValidateBagItVersion())
	require.Equal(t, 1, len(validator.Errors()))
	assert.Equal(t, "Profile requires a specific BagIt version, but no version is specified in bagit.txt", validator.Errors()[0])
}

func TestValidateAllowFetch(t *testing.T) {
	// fetch.txt not allowed and not present
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	validator.Profile.AllowFetchTxt = false
	assert.True(t, validator.ValidateAllowFetch())
	assert.Empty(t, validator.Errors())

	// Not allowed and present
	validator.Bag.TagFiles["fetch.txt"] = &bagit.File{}
	assert.False(t, validator.ValidateAllowFetch())
	require.NotEmpty(t, validator.Errors())
	assert.Equal(t, "Found fetch.txt, which BagIt profile says is not allowed.", validator.Errors()[0])

	// Allowed, but not present
	validator = getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	validator.Profile.AllowFetchTxt = true
	assert.True(t, validator.ValidateAllowFetch())
	assert.Empty(t, validator.Errors())

	// Alloed and present
	validator.Bag.TagFiles["fetch.txt"] = &bagit.File{}
	assert.True(t, validator.ValidateAllowFetch())
	assert.Empty(t, validator.Errors())
}

func TestValidateSerialization(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.True(t, validator.ValidateSerialization())
	assert.Empty(t, validator.Errors())

	validator.Profile.Serialization = constants.REQUIRED
	assert.True(t, validator.ValidateSerialization())
	assert.Empty(t, validator.Errors())

	validator.Profile.Serialization = constants.OPTIONAL
	assert.True(t, validator.ValidateSerialization())
	assert.Empty(t, validator.Errors())

	validator.Profile.Serialization = constants.FORBIDDEN
	assert.False(t, validator.ValidateSerialization())
	require.NotEmpty(t, validator.Errors())
	assert.Equal(t, "Serialization is forbidden, but bag is a single file", validator.Errors()[0])
}

func TestValidateSerializationFormat(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)

	// OK because profile says serialization is required
	// and we accept tar format.
	assert.True(t, validator.ValidateSerializationFormat())
	assert.Empty(t, validator.Errors())

	// Should fail, because the list of accepted formats
	// does not include .tar
	validator.Profile.AcceptSerialization = []string{".rar", ".7z", "zip"}
	assert.False(t, validator.ValidateSerializationFormat())
	require.NotEmpty(t, validator.Errors())
	assert.Equal(t, "Serialization format .tar is not in the Accept-Serialization list for this BagIt profile.", validator.Errors()[0])

	// Unrecognized format
	validator = getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	validator.Bag.Path = "path/to/unknown/format.fake"
	assert.False(t, validator.ValidateSerializationFormat())
	require.NotEmpty(t, validator.Errors())
	assert.Equal(t, "Unknown serialization type for format .fake.", validator.Errors()[0])

	// No serialization types specified in profile
	validator = getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	validator.Profile.AcceptSerialization = nil
	assert.False(t, validator.ValidateSerializationFormat())
	require.NotEmpty(t, validator.Errors())
	assert.Equal(t, "Bag is serialized, but profile does not specify accepted serializations.", validator.Errors()[0])
}

func TestValidateRequiredManifests(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.True(t, validator.ValidateRequiredManifests())
	assert.Empty(t, validator.Errors())

	validator.Profile.ManifestsRequired = append(validator.Profile.ManifestsRequired, "sha512")
	assert.False(t, validator.ValidateRequiredManifests())
	assert.NotEmpty(t, validator.Errors())
	assert.Equal(t, "Required manifest 'manifest-sha512.txt' is missing.", validator.Errors()[0])

	validator = getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	validator.Profile.TagManifestsRequired = append(validator.Profile.TagManifestsRequired, "sha512")
	assert.False(t, validator.ValidateRequiredManifests())
	assert.NotEmpty(t, validator.Errors())
	assert.Equal(t, "Required tag manifest 'tagmanifest-sha512.txt' is missing.", validator.Errors()[0])
}

func TestValidateTagFiles(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.True(t, validator.ValidateTagFiles())
	assert.Empty(t, validator.Errors())

	// Make sure it catches missing files.
	delete(validator.Bag.TagFiles, "aptrust-info.txt")
	assert.False(t, validator.ValidateTagFiles())
	require.NotEmpty(t, validator.Errors())
	assert.Equal(t, "Required tag file 'aptrust-info.txt' is missing.", validator.Errors()[0])

	// Make sure it catches missing tags.
	validator = getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	validator.ReadBag()
	validator.Bag.TagFiles["aptrust-info.txt"].ParsedData.DeleteByKey("Title")
	assert.False(t, validator.ValidateTagFiles())
	require.NotEmpty(t, validator.Errors())
	assert.Equal(t, "Required tag 'Title' is missing from file 'aptrust-info.txt'.", validator.Errors()[0])
}

func TestValidateTag(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	tagDef := validator.Profile.FindTagDef("aptrust-info.txt", "Access")
	assert.True(t, validator.ValidateTag(tagDef, []string{"Consortia"}))
	assert.Empty(t, validator.Errors())
	assert.True(t, validator.ValidateTag(tagDef, []string{"Institution"}))
	assert.Empty(t, validator.Errors())
	assert.True(t, validator.ValidateTag(tagDef, []string{"Restricted"}))
	assert.Empty(t, validator.Errors())

	// Value not explicitly allowed
	validator = getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.ValidateTag(tagDef, []string{"Inertia"}))
	require.NotEmpty(t, validator.Errors())
	assert.Equal(t, "In file 'aptrust-info.txt': Value 'Inertia' for tag 'Access' is not in list of allowed values (Consortia, Institution, Restricted)", validator.Errors()[0])

	// Missing required tag
	validator = getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.ValidateTag(tagDef, nil))
	require.NotEmpty(t, validator.Errors())
	assert.Equal(t, "Required tag 'Access' is missing from file 'aptrust-info.txt'.", validator.Errors()[0])

	// Empty tag where empty is not OK
	validator = getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.ValidateTag(tagDef, []string{"", "", ""}))
	require.NotEmpty(t, validator.Errors())
	assert.Equal(t, "Tag 'Access' in file 'aptrust-info.txt' cannot be empty.", validator.Errors()[0])

}

func TestValidateChecksums(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)

	assert.True(t, validator.ValidateChecksums())
	assert.Empty(t, validator.Errors())
}

// ---------- Test specific bags with specific issues --------- //

func TestValidateBadAccessBag(t *testing.T) {
	validator := getValidator(t, "example.edu.sample_bad_access.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.Validate())
	errors := validator.Errors()
	assert.Equal(t, 1, len(errors))
	assert.True(t, util.StringListContains(errors, "In file 'aptrust-info.txt': Value 'Hands Off!' for tag 'Access' is not in list of allowed values (Consortia, Institution, Restricted)"))
}

func TestValidateBadChecksumsBag(t *testing.T) {
	validator := getValidator(t, "example.edu.sample_bad_checksums.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.Validate())
	errors := validator.Errors()
	require.NotEmpty(t, errors)

	expected := []string{
		"Required tag 'Access' is missing from file 'aptrust-info.txt'.",
		"Digest for data/datastream-DC in manifest manifest-md5.txt: '44d85cf4810d6c6fe877BlahBlahBlah' does not match actual '44d85cf4810d6c6fe87750117633e461'",
		"Digest for data/datastream-descMetadata in manifest manifest-md5.txt: '4bd0ad5f85c00ce84a45BlahBlahBlah' does not match actual '4bd0ad5f85c00ce84a455466b24c8960'",
		"Digest for data/datastream-MARC in manifest manifest-md5.txt: '93e381dfa9ad0086dbe3BlahBlahBlah' does not match actual '93e381dfa9ad0086dbe3b92e0324bae6'",
		"Digest for data/datastream-RELS-EXT in manifest manifest-md5.txt: 'ff731b9a1758618f6cc2BlahBlahBlah' does not match actual 'ff731b9a1758618f6cc22538dede6174'",
	}

	for _, msg := range expected {
		assert.True(t, util.StringListContains(errors, msg), "Missing expected error: %s", msg)
	}
}

func TestValidateGoodBag(t *testing.T) {
	validator := getValidator(t, "example.edu.sample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.True(t, validator.Validate())
	errors := validator.Errors()
	require.Empty(t, errors)
}

func TestValidateMissingDataFileBag(t *testing.T) {
	validator := getValidator(t, "example.edu.sample_missing_data_file.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.Validate())
	errors := validator.Errors()
	require.Equal(t, 2, len(errors))

	expected := []string{
		"Required tag 'Access' is missing from file 'aptrust-info.txt'.",
		"File data/datastream-DC in manifest manifest-md5.txt is missing from the data directory",
	}

	for _, msg := range expected {
		assert.True(t, util.StringListContains(errors, msg), "Missing expected error: %s", msg)
	}
}

func TestValidateMissingAPTrustInfoBag(t *testing.T) {
	validator := getValidator(t, "example.edu.sample_no_aptrust_info.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.Validate())
	errors := validator.Errors()
	require.Equal(t, 1, len(errors))
	assert.Equal(t, "Required tag file 'aptrust-info.txt' is missing.", errors[0])
}

func TestValidateNoBagInfoBag(t *testing.T) {
	validator := getValidator(t, "example.edu.sample_no_bag_info.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.Validate())
	errors := validator.Errors()
	require.Equal(t, 2, len(errors))

	expected := []string{
		"Required tag file 'bag-info.txt' is missing.",
		"Required tag 'Access' is missing from file 'aptrust-info.txt'.",
	}
	for _, msg := range expected {
		assert.True(t, util.StringListContains(errors, msg), "Missing expected error: %s", msg)
	}
}

func TestValidateNoBagItBag(t *testing.T) {
	validator := getValidator(t, "example.edu.sample_no_bagit.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.Validate())
	errors := validator.Errors()
	require.Equal(t, 3, len(errors))

	expected := []string{
		"Cannot check bagit version because bagit.txt is missing.",
		"Required tag file 'bagit.txt' is missing.",
		"Required tag 'Access' is missing from file 'aptrust-info.txt'.",
	}
	for _, msg := range expected {
		assert.True(t, util.StringListContains(errors, msg), "Missing expected error: %s", msg)
	}
}

func TestValidateNoDataDirBag(t *testing.T) {
	validator := getValidator(t, "example.edu.sample_no_data_dir.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.Validate())
	errors := validator.Errors()
	assert.Equal(t, 4, len(errors))

	expected := []string{
		"File data/datastream-DC in manifest manifest-md5.txt is missing from the data directory",
		"File data/datastream-descMetadata in manifest manifest-md5.txt is missing from the data directory",
		"File data/datastream-MARC in manifest manifest-md5.txt is missing from the data directory",
		"File data/datastream-RELS-EXT in manifest manifest-md5.txt is missing from the data directory",
	}
	for _, msg := range expected {
		assert.True(t, util.StringListContains(errors, msg), "Missing expected error: %s", msg)
	}
}

func TestValidateNoMd5ManifestBag(t *testing.T) {
	validator := getValidator(t, "example.edu.sample_no_md5_manifest.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.Validate())
	errors := validator.Errors()
	require.Equal(t, 1, len(errors))
	assert.Equal(t, "Required manifest 'manifest-md5.txt' is missing.", errors[0])
}

func TestValidateNoTitleBag(t *testing.T) {
	validator := getValidator(t, "example.edu.sample_no_title.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.Validate())
	errors := validator.Errors()
	require.Equal(t, 1, len(errors))
	assert.Equal(t, "Tag 'Title' in file 'aptrust-info.txt' cannot be empty.", errors[0])
}

func TestValidateWrongFolderNameBag(t *testing.T) {
	validator := getValidator(t, "example.edu.sample_wrong_folder_name.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.Validate())
	errors := validator.Errors()
	require.Equal(t, 1, len(errors))
	assert.Equal(t, "Bag should untar to a single directory whose name matches the name of the tar file", errors[0])
}

func TestValidateBadTagSampleBag(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_bad.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.Validate())
	errors := validator.Errors()
	require.NotEmpty(t, errors)
	assert.Equal(t, 6, len(errors))

	expected := []string{
		"Tag 'Title' in file 'aptrust-info.txt' cannot be empty.",
		"In file 'aptrust-info.txt': Value 'acksess' for tag 'Access' is not in list of allowed values (Consortia, Institution, Restricted)",
		"Digest for data/datastream-descMetadata in manifest manifest-sha256.txt: 'This-checksum-is-bad-on-purpose.-The-validator-should-catch-it!!' does not match actual 'cf9cbce80062932e10ee9cd70ec05ebc24019deddfea4e54b8788decd28b4bc7'",
		"File data/file-not-in-bag in manifest manifest-sha256.txt is missing from the data directory",
		"Digest for custom_tags/tracked_tag_file.txt in tag manifest tagmanifest-sha256.txt: '0000000000000000000000000000000000000000000000000000000000000000' does not match actual '3f2f50c5bde87b58d6132faee14d1a295d115338643c658df7fa147e2296ccdd'",
		"Digest for custom_tags/tracked_tag_file.txt in tag manifest tagmanifest-md5.txt: '00000000000000000000000000000000' does not match actual 'dafbffffc3ed28ef18363394935a2651'",
	}
	for _, msg := range expected {
		assert.True(t, util.StringListContains(errors, msg), "Missing expected error: %s", msg)
	}
}

func TestValidateGoodTagSampleBag(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "aptrust_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.True(t, validator.Validate())
	assert.Empty(t, validator.Errors())
}

// Bag example.edu.tagsample_good.tar is valid according to the APTrust
// BagIt profile, but not valid according to the DPN profile. So although
// it passed in the test above, it should fail here.
func TestValidateAPTrustBagUsingDPNProfile(t *testing.T) {
	validator := getValidator(t, "example.edu.tagsample_good.tar", "dpn_bagit_profile_2.1.json")
	require.NotNil(t, validator)
	assert.False(t, validator.Validate())
	errors := validator.Errors()
	require.NotEmpty(t, errors)
	assert.Equal(t, 9, len(errors), "%v", errors)

	expected := []string{
		"Non-manifest file 'aptrust-info.txt' is not allowed in top-level directory when BagIt profile says AllowMiscTopLevelFiles is false.",
		"Non-manifest file 'custom_tag_file.txt' is not allowed in top-level directory when BagIt profile says AllowMiscTopLevelFiles is false.",
		"Non-manifest file 'junk_file.txt' is not allowed in top-level directory when BagIt profile says AllowMiscTopLevelFiles is false.",
		"Required tag 'Organization-Address' is missing from file 'bag-info.txt'.",
		"Required tag 'Contact-Name' is missing from file 'bag-info.txt'.",
		"Required tag 'Contact-Phone' is missing from file 'bag-info.txt'.",
		"Required tag 'Contact-Email' is missing from file 'bag-info.txt'.",
		"Required tag 'Bag-Size' is missing from file 'bag-info.txt'.",
		"Required tag file 'dpn-tags/dpn-info.txt' is missing.",
	}

	for _, msg := range expected {
		assert.True(t, util.StringListContains(errors, msg), "Missing expected error: %s", msg)
	}
}

func TestValidateUntarredGoodBag(t *testing.T) {
	// Load the APTrust BagIt Profile
	profilePath, err := testutil.GetPathToTestProfile("aptrust_bagit_profile_2.1.json")
	require.Nil(t, err)
	aptrustProfile, err := bagit.LoadBagItProfile(profilePath)
	require.Nil(t, err)

	// Untar a bag that we know is good.
	pathToTarredBag, err := testutil.GetPathToTestBag("example.edu.tagsample_good.tar")
	require.Nil(t, err)
	tempDir, pathToUntarredBag, err := testutil.UntarTestBag(pathToTarredBag)
	defer os.RemoveAll(tempDir)
	require.Nil(t, err)

	// Create the bag object and the vaidator.
	bag := bagit.NewBag(pathToUntarredBag)
	validator := bagit.NewValidator(bag, aptrustProfile)
	require.NotNil(t, validator)

	// Turn this off temporarily, so we can validate.
	validator.Profile.Serialization = constants.OPTIONAL

	// Make sure it validates. This test ensures that the underlying
	// FileSystemIterator inside the Validator behaves just like the
	// TarFileIterator tested in all the cases above.
	assert.True(t, validator.Validate())
	assert.Empty(t, validator.Errors())
}

func TestValidateUntarredBadBag(t *testing.T) {
	// Load the APTrust BagIt Profile
	profilePath, err := testutil.GetPathToTestProfile("aptrust_bagit_profile_2.1.json")
	require.Nil(t, err)
	aptrustProfile, err := bagit.LoadBagItProfile(profilePath)
	require.Nil(t, err)

	// Untar a bag that we know is bad.
	pathToTarredBag, err := testutil.GetPathToTestBag("example.edu.tagsample_bad.tar")
	require.Nil(t, err)
	tempDir, pathToUntarredBag, err := testutil.UntarTestBag(pathToTarredBag)
	defer os.RemoveAll(tempDir)
	require.Nil(t, err)

	// Create the bag object and the vaidator.
	bag := bagit.NewBag(pathToUntarredBag)
	validator := bagit.NewValidator(bag, aptrustProfile)
	require.NotNil(t, validator)

	// Turn this off temporarily, so we can validate.
	validator.Profile.Serialization = constants.OPTIONAL

	// We should find the same errors as in TestValidateBadTagSampleBag.
	assert.False(t, validator.Validate())
	errors := validator.Errors()
	require.NotEmpty(t, errors)

	expected := []string{
		"Tag 'Title' in file 'aptrust-info.txt' cannot be empty.",
		"In file 'aptrust-info.txt': Value 'acksess' for tag 'Access' is not in list of allowed values (Consortia, Institution, Restricted)",
		"Digest for data/datastream-descMetadata in manifest manifest-sha256.txt: 'This-checksum-is-bad-on-purpose.-The-validator-should-catch-it!!' does not match actual 'cf9cbce80062932e10ee9cd70ec05ebc24019deddfea4e54b8788decd28b4bc7'",
		"File data/file-not-in-bag in manifest manifest-sha256.txt is missing from the data directory",
		"Digest for custom_tags/tracked_tag_file.txt in tag manifest tagmanifest-sha256.txt: '0000000000000000000000000000000000000000000000000000000000000000' does not match actual '3f2f50c5bde87b58d6132faee14d1a295d115338643c658df7fa147e2296ccdd'",
		"Digest for custom_tags/tracked_tag_file.txt in tag manifest tagmanifest-md5.txt: '00000000000000000000000000000000' does not match actual 'dafbffffc3ed28ef18363394935a2651'",
	}
	for _, msg := range expected {
		assert.True(t, util.StringListContains(errors, msg), "Missing expected error: %s", msg)
	}
}
