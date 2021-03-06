const electron = require('electron');
const app = (process.type === 'renderer') ? electron.remote.app : electron.app;
const fs = require('fs');
const log = require('electron-log/main'); // requiring 'electron-log' leaves transport undefined
const path = require('path');
const { Util } = require('./util');
const zlib = require('zlib');


const megabyte = 1048576;

log.transports.file.format = "[{y}-{m}-{d} {h}:{i}:{s}.{ms}] [{level}] {text}";
log.transports.console.format = "{text}";
log.transports.file.maxSize = 1 * megabyte;

if (Util.isDevMode()) {
    log.transports.file.level = 'debug';
    log.transports.console.level = false;
} else {
    // For early distribution builds, set log level to debug.
    // We'll set this back to 'info' when we're more stable.
    // Log Level should be an app setting with options 'info' and 'debug'.
    log.transports.file.level = 'debug';
    log.transports.console.level = false;
}

log.filename = function() {
    return log.transports.file.findLogPath();
}

// This dumps a gzipped copy of the log to the user's desktop
// and returns the name of the file.
log.zip = function() {
    var gzip = zlib.createGzip();
    var infile = fs.createReadStream(log.filename());
    var timestamp = new Date().getTime();
    var fname = `DartLog_${timestamp}.txt.gz`
    var outfile = fs.createWriteStream(path.join(app.getPath('desktop'), fname));
    infile.pipe(gzip).pipe(outfile);
    return fname;
}

log.contents = function(callback) {
    fs.readFile(log.filename(), 'utf8', callback);
}

// Use log.info("message")
//     log.warn("message")
//     log.error("message")
//     log.debug("message")

module.exports = log;
