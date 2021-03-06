const async = require('async');
const { BagItFile } = require('./bagit_file');
const constants = require('./constants');
const crypto = require('crypto');
const EventEmitter = require('events');
const fs = require('fs');
const log = require('../core/log');
const { ManifestParser } = require('./manifest_parser');
const os = require('os');
const path = require('path');
const stream = require('stream');
const { TagFileParser } = require('./tag_file_parser');
const tar = require('tar-stream');
const { Util } = require('../core/util');

class Validator {

    // pathToBag is the absolute path the the bag (dir or tar file)
    //
    // profile is the BagItProfile that describes what consititutes
    // a valid bag.
    //
    // emitter is an instance of EventEmitter that listens for the
    // following events:
    //
    // validateStart - fires when validation starts
    //               - function(message [string])
    // validateComplete - fires when validation completes
    //               - function (succeeded [bool], message [string])
    //
    // You can pass a null emitter if you don't care to listen to events.
    constructor(pathToBag, profile, emitter) {
        this.pathToBag = pathToBag;
        this.profile = profile;
        this.emitter = emitter || new EventEmitter();
        this.bagName = path.basename(pathToBag, '.tar');

        // files is a hash of BagItFiles, where the file's path
        // within the bag (relPath) is the key, and the BagItFile
        // object is the value. The hash makes it easy to get files
        // by relative path within the archive (e.g. data/photos/img.jpg).
        this.files = {};

        // These arrays contains the same BagItFile objects as the
        // files hash above, but these are organized by type.
        this.payloadFiles = [];
        this.payloadManifests = [];
        this.tagManifests = [];
        this.tagFiles = [];

        // Some profiles prohibit top-level directories other
        // than /data, and some prohibit top-level files that
        // are not required manifests or required tag files.
        this.topLevelDirs = [];
        this.topLevelFiles = [];

        this.errors = [];
    }

    // validate validates the bag
    validate() {
        // Make sure untarred name matches tarred name
        // Gather checksums on all files
        // Validate checksums
        // Validate no extra or missing files
        // Ensure required tag files
        // Ensure required tags with legal values
        this.emitter.emit('validateStart', `Validating ${this.pathToBag}`);
        if (this.pathToBag.endsWith('.tar')) {
            this._readFromTar();
        } else {
            this._readFromDir();
        }
    }

    _readFromTar() {
        log.debug(`Validator is reading from tar file at ${this.pathToBag}`);
        var validator = this;
        var extract = tar.extract();
        var bagNamePrefix = this.bagName + '/';
        var addedBagFolderNameError = false;
        extract.on('entry', function(header, stream, next) {
            // header is the tar header
            // stream is the content body (might be an empty stream)
            // call next when you are done with this entry
            var stats = {
                size: header.size,
                mode: header.mode,
                uid: header.uid,
                gid: header.gid,
                mtimeMs: header.mtime
            }

            if (!addedBagFolderNameError && !header.name.startsWith(bagNamePrefix)) {
                addedBagFolderNameError = true;
                var actualFolder = header.name.split('/')[0];
                this.errors.push(`Bag must untar to a folder called '${this.bagName}', not '${actualName}'`);
            }

            // If bag untars to the wrong directory, it's not valid, and there's
            // no use parsing the rest of it. This might save us running checksums
            // on many GB of data.
            if (!addedBagFolderNameError) {
                var absSourcePath = header.name;
                var relDestPath = header.name.replace(bagNamePrefix, '');
                var bagItFile = new BagItFile(absSourcePath, relDestPath, stats);
                validator.readFile(bagItFile, stream);
            }

            stream.on('end', function() {
                next() // ready for next entry
            })
        })

        extract.on('error', function(err) {
            validator.emitter.emit('error', err);
        });

        extract.on('finish', function() {
            // all entries read
            validator.validateTopLevelDirs();
            validator.validateTopLevelFiles();
            validator.validateRequiredManifests();
            validator.validateRequiredTagManifests();
            validator.validateManifests(validator.payloadManifests);
            validator.validateManifests(validator.tagManifests);
            validator.validateNoExtraneousPayloadFiles();
            validator.validateTags();

            if (validator.errors.length == 0) {
                validator.emitter.emit('validateComplete', true, `Bag ${validator.pathToBag} is valid`);
            } else {
                validator.emitter.emit('validateComplete', false, validator.errors.join("\n"));
            }
        })

        // PT #155978872
        // fs.readFileSync can't read files over 2GB.
        // Fix this with read or readSync or fs.createReadStream.
        // extract.end(fs.readFile(this.pathToBag));
        fs.createReadStream(this.pathToBag).pipe(extract)
    }

    readFile(bagItFile, stream) {
        log.debug(`Running checksums on ${bagItFile.relDestPath}`);
        this._addFile(bagItFile);
        var pipes = this._getCryptoHashes(bagItFile)
        if (bagItFile.fileType == constants.PAYLOAD_FILE) {
            // No need for additional piping, just need crypto hashes.
        } else if (bagItFile.fileType == constants.PAYLOAD_MANIFEST) {
            var manifestParser = new ManifestParser(bagItFile);
            pipes.push(manifestParser.stream);
        } else if (bagItFile.fileType == constants.TAG_MANIFEST) {
            var manifestParser = new ManifestParser(bagItFile);
            pipes.push(manifestParser.stream);
        } else if (bagItFile.fileType == constants.TAG_FILE) {
            var tagFileParser = new TagFileParser(bagItFile);
            pipes.push(tagFileParser.stream);
        } else {
            pipes = null;
            throw `Unkonwn file type: ${bagItFile.fileType}`
        }
        for (var p of pipes) {
            stream.pipe(p);
        }
    }

    _addFile(bagItFile) {
        this.files[bagItFile.relDestPath] = bagItFile;
        switch (bagItFile.fileType) {
            case constants.PAYLOAD_FILE:
              this.payloadFiles.push(bagItFile);
              break;
            case constants.PAYLOAD_MANIFEST:
              this.payloadManifests.push(bagItFile);
              break;
            case constants.TAG_MANIFEST:
              this.tagManifests.push(bagItFile);
              break;
            default:
              this.tagFiles.push(bagItFile);
        }
        // Keep a list of top-level directory and file names.
        // tar files use forward slash, even on Windows
        // parts[0] should match bag name.
        var parts = bagItFile.relDestPath.split('/');
        var name = parts[0];
        if (parts.length > 1) {
            if (!Util.listContains(this.topLevelDirs, name)) {
                this.topLevelDirs.push(name);
            }
        } else {
            if (!Util.listContains(this.topLevelFiles, name)) {
                this.topLevelFiles.push(name);
            }
        }
    }

    _getCryptoHashes(bagItFile) {
        var hashes = [];
        for (var algorithm of this.profile.manifestsRequired) {
            var hash = crypto.createHash(algorithm);
            hash.setEncoding('hex');
            hash.on('finish', function() {
                hash.end();
                bagItFile.checksums[algorithm] = hash.read();
            });
            hashes.push(hash);
        }
        return hashes;
    }

    validateManifests(manifests) {
        for(var manifest of manifests) {
            log.debug(`Validating ${manifest.relDestPath}`);
            var basename = path.basename(manifest.relDestPath, '.txt');
            var algorithm = basename.split('-')[1];
            for (var filename of manifest.keyValueCollection.keys()) {
                var bagItFile = this.files[filename];
                if (!bagItFile) {
                    this.errors.push(`File ${filename} in ${manifest.relDestPath} is missing from payload.`);
                    continue;
                }
                var checksumInManifest = manifest.keyValueCollection.first(filename);
                var calculatedChecksum = bagItFile.checksums[algorithm];
                if (checksumInManifest != calculatedChecksum) {
                    this.errors.push(`Checksum for '${filename}': expected ${checksumInManifest}, got ${calculatedChecksum}`);
                }
            }
        }
    }

    validateNoExtraneousPayloadFiles() {
        log.debug("Checking for extraneous payload files");
        for(var manifest of this.payloadManifests) {
            for (var f of this.payloadFiles) {
                //console.log("Payload file " + f.relDestPath)
                if (!manifest.keyValueCollection.first(f.relDestPath)) {
                    this.errors.push(`Payload file ${f.relDestPath} not found in ${manifest.relDestPath}`);
                }
            }
        }
    }

    validateTopLevelDirs() {
        log.debug("Validating top-level directories");
        var exceptions = ['data']; // data dir is always required
        for (var f of this.profile.requiredTagFileNames()) {
            var requiredTagDir = f.split('/', 1);
            exceptions.push(requiredTagDir);
        }
        if (!this.profile.allowMiscTopLevelDirectories) {
            for (var dir of this.topLevelDirs) {
                if (!Util.listContains(exceptions, dir)) {
                    this.errors.push(`Profile prohibits top-level directory ${dir}`);
                }
            }
        }
    }

    validateTopLevelFiles() {
        log.debug("Validating top-level files");
        if (!this.profile.allowMiscTopLevelFiles) {
            var exceptions = this.profile.requiredTagFileNames();
            for (var alg of this.profile.manifestsRequired) {
                exceptions.push(`manifest-${alg}.txt`);
            }
            for (var alg of this.profile.tagManifestsRequired) {
                exceptions.push(`tagmanifest-${alg}.txt`);
            }
            for (var name of this.topLevelFiles) {
                if (name == 'fetch.txt') {
                    // This one has its own rule
                    if (!this.profile.allowFetchTxt) {
                        this.errors.push(`Bag contains fetch.txt file, which profile prohibits.`);
                    }
                    continue;
                }
                if (!Util.listContains(exceptions, name)) {
                    this.errors.push(`Profile prohibits top-level file ${name}`);
                }
            }
        }
    }

    validateRequiredManifests() {
        log.debug("Checking for presence of required manifests");
        for (var alg of this.profile.manifestsRequired) {
            var name = `manifest-${alg}.txt`
            if(!this.files[name]) {
                this.errors.push(`Bag is missing required manifest ${name}`);
            }
        }
    }

    validateRequiredTagManifests() {
        log.debug("Checking for presence of required tag manifests");
        for (var alg of this.profile.tagManifestsRequired) {
            var name = `tagmanifest-${alg}.txt`
            if(!this.files[name]) {
                this.errors.push(`Bag is missing required manifest ${name}`);
            }
        }
    }

    // Validate that all required tags are present and have legal values.
    // Among other things, this verfies that the BagIt-Version is correct.
    validateTags() {
        var requiredTags = this.profile.tagsGroupedByFile();
        for (var filename of Object.keys(requiredTags)) {
            log.debug(`Checking required tags in ${filename}`);
            var tagFile = this.files[filename];
            if (!tagFile) {
                this.errors.push(`Required tag file ${filename} is missing`);
                continue;
            }
            if (tagFile.keyValueCollection == null) {
                this.errors.push(`Tag file ${filename} has no data`);
                continue;
            }
            var tagsRequiredForThisFile = requiredTags[filename];
            for (var tagDef of tagsRequiredForThisFile) {
                //console.log(`Checking ${filename} -> ${tagDef.tagName}`);
                var parsedTagValues = tagFile.keyValueCollection.all(tagDef.tagName);
                //console.log(parsedTagValues);
                if (parsedTagValues == null) {
                    // Tag was not present at all.
                    if (tagDef.required) {
                        this.errors.push(`Required tag ${tagDef.tagName} is missing from ${filename}`);
                    }
                    continue;
                }
                for (var value of parsedTagValues) {
                    if (value == '' && tagDef.emptyOk) {
                        continue;
                    }
                    if (Array.isArray(tagDef.values) && tagDef.values.length > 0) {
                        if (!Util.listContains(tagDef.values, value)) {
                            this.errors.push(`Tag ${tagDef.tagName} in ${filename} contains illegal value ${value}. [Allowed: ${tagDef.values.join(', ')}]`);
                        }
                    }
                }
            }
        }
    }

    // callback is the function to call when validation is complete.
    _readFromDir(callback) {
        log.debug(`Validator is trying to read from directory at ${this.pathToBag} - BUT THIS IS NOT YET IMPLEMENTED`);
        throw "Reading bag from a directory is not yet implemented. It's tar only for now."
    }
}

module.exports.Validator = Validator;
