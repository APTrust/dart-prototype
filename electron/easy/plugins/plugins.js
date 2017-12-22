const requireDir = require('require-dir');
const path = require('path');

const PackageProviders = requireDir(path.resolve("electron/easy/plugins/packaging"));
const StorageProviders = requireDir(path.resolve("electron/easy/plugins/storage"));

// Returns a list of StorageProvider protocols.
// For example, ["s3", "ftp", "rsync"]
function listStorageProviders() {
    var protocols = [];
    for(var moduleName in StorageProviders) {
        var provider = StorageProviders[moduleName];
        protocols.push(provider.protocol);
    }
    return protocols;
}

// Returns a map of PackageProviders in which the keys
// are file formats and values are mimetypes. For example:
//
// {
//   'tar': 'application/x-tar',
//   'zip': 'application/zip',
//   'bagit': ''
// }
//
// Note that there is no mimetype for bagit.
function listPackageProviders() {
    var formats = {};
    for(var moduleName in PackageProviders) {
        var provider = PackageProviders[moduleName];
        formats[provider.format] = provider.formatMimeType;
    }
    return formats;
}

// Returns the storage provider that supports the specified
// protocol. Protocol can be 's3', 'ftp', etc.
function getStorageProviderByProtocol(protocol) {
    for(var moduleName in StorageProviders) {
        var provider = StorageProviders[moduleName];
        if (provider.protocol == protocol) {
            return provider;
        }
    }
    return null;
}

// Returns the package provider that supports the specified
// format. Format can be 'tar', 'bagit', 'bzip', etc.
function getPackageProviderByFormat(format) {
    for(var moduleName in PackageProviders) {
        var module = PackageProviders[moduleName];
        if (module.format == format) {
            return module.Provider;
        }
    }
    return null;
}

// Returns the package provider that supports the specified
// mime-type. Param mimetype is something like 'application/x-zip',
// etc. There is no mime type for bagit, so use
// getPackageProviderByFormat for the bagit format.
function getPackageProviderByMimeType(mimetype) {
    for(var moduleName in PackageProviders) {
        var module = PackageProviders[moduleName];
        if (module.formatMimeType == mimetype) {
            return module.Provider;
        }
    }
    return null;
}

module.exports.listStorageProviders = listStorageProviders;
module.exports.listPackageProviders = listPackageProviders;
module.exports.getStorageProviderByProtocol = getStorageProviderByProtocol;
module.exports.getPackageProviderByFormat = getPackageProviderByFormat;
module.exports.getPackageProviderByMimeType = getPackageProviderByMimeType;