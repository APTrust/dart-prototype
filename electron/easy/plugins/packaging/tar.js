const path = require('path');
const OperationResult = require(path.resolve('electron/easy/operation_result'));

const format = "tar";
const formatMimeType = "application/x-tar";

class Tar {

    /**
     * Custom packager.
     * @constructor
     * @param {object} job - The job object. See easy/job.js.
     * @returns {object} - A new custom packager.
     */
    constructor(job) {
        this.job = job;
        // ... code ...
    }

    /**
     * Assembles all job.files into a package (e.g. a zip file,
     * tar file, rar file, etc.).
     * @returns {object} - An instance of OperationResult.
     * See easy/operation_result.js.
     */
    package() {
        var result = new OperationResult();
        try {
            // ... code ...
        } catch (ex) {
            // ... code ...
        }
        return result;
    }
}

module.exports.Provider = Tar;
module.exports.format = format;
module.exports.formatMimeType = formatMimeType;