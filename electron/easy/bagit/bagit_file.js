const constants = require('./constants');

module.exports = class BagItFile {

    constructor(absSourcePath, relDestPath, stats) {
        this.absSourcePath = absSourcePath;
        this.relDestPath = relDestPath;
        this.stats = stats;
        this.fileType = BagItFile.getFileType(relDestPath);
        // checksums:
        // key = algorithm name ('md5', 'sha256', etc.)
        // value = digest
        this.checksums = {};
    }

    // Returns the manifest entry for the specified algorithm,
    // or throws an exception if the checksum for that algorithm
    // is not present. The format of the returned string is suitable
    // for printing into a payload manifest or tag manifest.
    getManifestEntry(algorithm) {
        var checksum = this.checksums[algorithm];
        if (checksum === undefined || !checksum) {
            throw `No ${algorithm} digest for ${this.absSourcePath}`;
        }
        return `${checksum} ${this.relDestPath}`;
    }

    static getFileType(relDestPath) {
        if (relDestPath.startsWith('data/')) {
            return constants.PAYLOAD_FILE;
        } else if (relDestPath.startsWith('manifest-')) {
            return constants.PAYLOAD_MANIFEST;
        } else if (relDestPath.startsWith('tagmanifest-')) {
            return constants.TAG_MANIFEST;
        }
        return constants.TAG_FILE;
    }
}