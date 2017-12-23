const fs = require('fs');
const path = require('path');
const { spawn } = require('child_process');
const NEWLINE = require('os').EOL;
const dateFormat = require('dateformat');
const AppSetting = require(path.resolve('electron/easy/core/app_setting'));
const BagItProfile = require(path.resolve('electron/easy/core/bagit_profile'));
const BagItProfileInfo = require(path.resolve('electron/easy/core/bagit_profile_info'));
const Choice = require(path.resolve('electron/easy/core/choice'));
const Const = require(path.resolve('electron/easy/core/constants'));
const Field = require(path.resolve('electron/easy/core/field'));
const Form = require(path.resolve('electron/easy/core/form'));
const JobOptions = require(path.resolve('electron/easy/core/job_options'));
const OperationResult = require(path.resolve('electron/easy/core/operation_result'));
const Plugins = require(path.resolve('electron/easy/plugins/plugins'));
const QuickStat = require(path.resolve('electron/easy/core/quick_stat'));
const StorageService = require(path.resolve('electron/easy/core/storage_service'));
const Util = require(path.resolve('electron/easy/core/util'));
const ValidationResult = require(path.resolve('electron/easy/core/validation_result'));

const Store = require('electron-store');
var db = new Store({name: 'jobs'});


const macJunkFile = /._DS_Store$|.DS_Store$/i;
const dotFile = /\/\.[^\/]+$|\\\.[^\\]+$/;
const dotKeepFile = /\/\.keep$|\\\.keep$/i;

var kb = 1024;
var mb = 1024 * kb;
var gb = 1024 * mb;
var tb = 1024 * gb;


module.exports = class Job {
	constructor() {
		this.id = Util.uuid4();
		this.bagName = "";
		this.baggingDirectory = "";
		this.files = [];
		this.bagItProfile = null;
		this.storageServices = [];
		this.options = new JobOptions();
		this.operationResults = [];
		this.created = null;
		this.updated = null;

		var setting = AppSetting.findByName("Bagging Directory");
		if (setting != null) {
			this.baggingDirectory = setting.value;
		}
	}
	objectType() {
		return 'Job';
	}

	displayName() {
		var dateString = this.displayDateCreated();
		var name = `Job created ${dateString}`
		if (this.bagName != null && this.bagName.trim() != "") {
			name = this.bagName;
		}
		return name;
	}

	displayDateCreated() {
		return dateFormat(this.created, 'longDate') + " " + dateFormat(this.created, 'shortTime');
	}

	displayDateUpdated() {
		return dateFormat(this.updated, 'longDate') + " " + dateFormat(this.updated, 'shortTime');
	}

	displayTitle() {
		var title = "";
		var tagNames = ["Title", "Description", "Internal-Sender-Identifier",
						"Local-ID",  "External-Description", "Internal-Sender-Description",
						"External-Identifier"]
		if (this.bagItProfile != null) {
			var tag = this.bagItProfile.firstTagWithMatchingName(tagNames);
			if (tag != null) {
				title = tag.userValue;
			}
		}
		return title;
	}

	clearFiles() {
		this.files = [];
	}

	hasFile(filepath) {
		if (!this.hasFiles) {
			return false;
		}
		if (!Job.shouldIncludeFile(filepath, this.options)) {
			return false;
		}
		var included = false;
		for (var f of this.files) {
			if (filepath.startsWith(f)) {
				included = true;
				break;
			}
		}
		return included;
	}

	hasFiles() {
		return this.files != null && this.files.length > 0;
	}

	static shouldIncludeFile(filepath, options) {
		// Return false if this file should be filtered out of the package.
		var isMacJunk = filepath.match(macJunkFile);
		var isHidden = filepath.match(dotFile);
		var isDotKeep = filepath.match(dotKeepFile);
		var skipMacJunk = (options.skipDSStore || options.skipDotKeep || options.skipHiddenFiles);

		if (isMacJunk && skipMacJunk) {
			return false;
		}
		else if (isDotKeep && options.skipDotKeep) {
			return false;
		}
		else if (isHidden && options.skipHiddenFiles) {
			return false;
		}
		return true;
	}

	validate() {
		var result = new ValidationResult();
		if (this.files == null || this.files.length == 0) {
			result.errors["files"] = ["This job has no files."];
		}
		if (this.bagItProfile == null && (this.storageServices == null || this.storageServices.length == 0)) {
			result.errors["general"] = ["This job must have either a BagIt Profile, or a Storage Service, or both."];
		}
		if (this.bagItProfile != null) {
			if (this.baggingDirectory == "" || this.baggingDirectory == null) {
				result.errors["baggingDirectory"] = ["You must specify a bagging directory."];
			}
			let errors = []
			var profileResult = this.bagItProfile.validate();
			if (!profileResult.isValid()) {
				for (var k of Object.keys(profileResult.errors)) {
					errors.push(profileResult.errors[k])
				}
			}
			for (var tag of this.bagItProfile.requiredTags) {
				for (var err of tag.validateForJob()) {
					errors.push(err);
				}
			}
			if (errors.length > 0) {
				result.errors['bagItProfile'] = errors;
			}
		}
		if (this.storageServices != null) {
			let errors = []
			for (var ss of this.storageServices) {
				var ssResult = ss.validate();
				if (!ssResult.isValid()) {
					for (var k of Object.keys(ssResult.errors)) {
						errors.push(profileResult.errors[k])
					}
				}
			}
			if (errors.length > 0) {
				result.errors['storageServices'] = errors;
			}
		}
		return result;
	}

	resetFileOptions() {
		this.options.skipDSStore = true;
		this.options.skipHiddenFiles = false;
		this.options.skipDotKeep = false;
	}

	toPackagingForm() {
		var availableProfiles = Util.sortByName(BagItProfile.getStore());
		var profileId = null;
		if (this.bagItProfile != null) {
			profileId = this.bagItProfile.id;
		}
		var form = new Form();
		form.fields['bagName'] = new Field("bagName", "bagName", "Bag Name", this.bagName);
		form.fields['bagName'].help = "Provide a name for the bag you want to create. You can leave this blank if you're not creating a bag.";
		form.fields['baggingDirectory'] = new Field("baggingDirectory", "baggingDirectory", "Bagging Directory", this.baggingDirectory);
		form.fields['baggingDirectory'].help = "Where should the bag be assembled?";
		form.fields['profile'] = new Field("profile", "profile", "Packaging", "");
		form.fields['profile'].help = "Select a packaging format, or None if you just want to send files to the storage area as-is.";
		var choices = Choice.makeList(availableProfiles, profileId, true);
		choices[0].value = "";
		choices[0].label = "None";
		form.fields['profile'].choices = choices;
		return form;
	}

	setTagValuesFromForm() {
		if (this.bagItProfile == null) {
			return;
		}
		for (var input of $("#jobTagsForm .form-control")) {
			var id = $(input).attr('id');
			var tag = this.bagItProfile.findTagById(id);
			if (tag != null) {
				tag.userValue = $(input).val();
			}
		}
	}

	toStorageServiceForm() {
		var availableServices = Util.sortByName(StorageService.getStore());
		var form = new Form();
		form.fields['storageServices'] = new Field("storageServices", "storageServices", "Storage Services", this.storageServices);
		var selectedIds = this.storageServices.map(ss => ss.id);
		form.fields['storageServices'].choices = Choice.makeList(availableServices, selectedIds, false);
		return form;
	}

	setStorageServicesFromForm() {
		this.storageServices = [];
		for (var input of $("input[name=storageServices]:checked")) {
			var service = StorageService.find($(input).val());
			this.storageServices.push(service);
		}
	}

	save() {
		if (this.created == null) {
			this.created = new Date().toJSON();
		}
		this.updated = new Date().toJSON();
		return db.set(this.id, this);
	}

	static find(id) {
		var job = null;
		var obj = db.get(id);
		if (obj != null) {
			job = new Job();
			Object.assign(job, obj);
		}
		job.options = new JobOptions();
		Object.assign(job.options, obj.options);
		if (obj.bagItProfile != null) {
			job.bagItProfile = BagItProfile.toFullObject(obj.bagItProfile);
		}
		for (var i=0; i < obj.storageServices.length; i++) {
			var ss = new StorageService();
			Object.assign(ss, obj.storageServices[i]);
			job.storageServices[i] = ss;
		}
		for (var i=0; i < obj.operationResults.length; i++) {
			var result = new OperationResult();
			Object.assign(result, obj.operationResults[i]);
			job.operationResults[i] = result;
		}
		return job;
	}

	delete() {
		db.delete(this.id);
		return this;
	}

	getStore() {
		return db.store;
	}

	static list() {
		var items = Util.sortByCreated(Job.getStore());
		for (var i = 0; i < items.length; i++) {
			var item = items[i];
			var job = new es.Job();
			Object.assign(job, item)
			job.options = new JobOptions();
			Object.assign(job.options, item.options);
			if (item.bagItProfile != null) {
				job.bagItProfile = BagItProfile.toFullObject(item.bagItProfile);
			}
			items[i] = job;
		}
		return items;
	}

	fileOptionsChanged() {
		this.options.skipDSStore = $('#filesSkipDSStore').prop('checked');
		this.options.skipHiddenFiles = $('#filesSkipHidden').prop('checked');
		this.options.skipDotKeep = $('#filesSkipDotKeep').prop('checked');
		var job = this;
		$.each($("tr[data-object-type='File']"), function(index, row) {
			var filepath = $(row).data('filepath');
			job.deleteFile($(row).find('td').first());
			job.addFile(filepath);
		});
	}

	// We call this when we load an existing job, so the list of
	// files, file sizes, etc. shows up in the UI.
	setFileListUI() {
		var files = this.files.slice();
		this.files = [];
		for(var filepath of files) {
			this.addFile(filepath);
		}
	}

	addFile(filepath) {
		$('#filesPanel').show()
		$('#fileWarningContainer').hide();
		if (this.hasFile(filepath)) {
			$('#fileWarning').html(filepath + ' has already been added')
			$('#fileWarningContainer').show();
			return
		}
		var stat = fs.statSync(filepath)
		var row = $(getTableRow(filepath, stat.isDirectory()))
		row.insertBefore('#fileTotals')
		var job = this;

		var dirCallback = function() { updateStats(row, '.dirCount', 1) };
		var fileCallback = function(stats) { updateFileStats(stats, row) };
		var shouldIncludeCallback = function(filepath) { return Job.shouldIncludeFile(filepath, job.options); };
		var quickStat = new QuickStat(shouldIncludeCallback, fileCallback, dirCallback);
		fs.stat(filepath, function(err, stats) {
			quickStat.statPath(err, stats, filepath);
		});

		this.files.push(filepath)
		$('#btnJobPackagingDiv').show();
	}

	deleteFile(cell) {
		$('#fileWarningContainer').hide();
		var row = $(cell).parent('tr')
		var filepath = $(row).data('filepath')
		var removeIndex = this.files.indexOf(filepath);
		if (removeIndex > -1) {
			this.files.splice(removeIndex, 1);
		}
		updateStatsForDeletion(row);
		$(row).remove()
		if (!this.hasFiles()) {
			$('#btnJobPackagingDiv').hide();
		}
	}

	static getStore() {
		return db.store;
	}

	findResult(operation) {
		var result = null;
		for (var r of this.operationResults) {
			if (r.operation == operation) {
				result = r;
				break;
			}
		}
		return result;
	}

	// Run this job
	run() {
		var decoder = new TextDecoder("utf-8");
		var fileCount = 0;

		if (this.bagItProfile != null) {

			var job = this;
			var result = this.findResult("package");
			if (result == null) {
				result = new OperationResult("package");
				job.operationResults.push(result);
			}
			result.reset();
			result.attemptNumber += 1;
			result.started = (new Date()).toJSON();

			$('#jobRun').show();

			// Start the bagger executable
			var baggerProgram = path.resolve("apps/apt_create_bag/apt_create_bag");
			var bagger = spawn(baggerProgram, [ "--stdin" ]);

			bagger.on('error', (err) => {
				$("#jobError").show();
				$("#jobError").append(err + "<br/>");
				result.error += err + NEWLINE;
			});

			bagger.on('exit', function (code, signal) {
				result.info += `Bagger exited with code ${code} and signal ${signal}`;
				result.completed = (new Date()).toJSON();
				job.save(); // save job with OperationResult
			});

			bagger.stdout.on('data', (data) => {
				var lines = decoder.decode(data).split(NEWLINE);
				for (var line of lines) {
					if (line.startsWith('Adding')) {
						fileCount += 1;
						$("#jobRunFiles .message").text(line);
					} else if (line.startsWith('Writing')) {
						$("#jobRunFiles .message").text(`Added ${fileCount} files`);
						$("#jobRunFiles").removeClass("alert-info");
						$("#jobRunFiles").addClass("alert-success");
						$("#jobRunFiles .glyphicon").removeClass("glyphicon-hand-right");
						$("#jobRunFiles .glyphicon").addClass("glyphicon-thumbs-up");
						$("#jobPackage").show()
						$("#jobPackage .message").text(line);
					} else if (line.startsWith('Validating')) {
						$("#jobPackage").removeClass("alert-info");
						$("#jobPackage").addClass("alert-success");
						$("#jobPackage .glyphicon").removeClass("glyphicon-hand-right");
						$("#jobPackage .glyphicon").addClass("glyphicon-thumbs-up");
						$("#jobValidate").show();
						$("#jobValidate .message").html(line);
					} else if (line.startsWith('Bag at') && line.endsWith("is valid")) {
						$("#jobValidate").removeClass("alert-info");
						$("#jobValidate").addClass("alert-success");
						$("#jobValidate .glyphicon").removeClass("glyphicon-hand-right");
						$("#jobValidate .glyphicon").addClass("glyphicon-thumbs-up");
						$("#jobValidate").show();
						$("#jobValidate .message").append("<br/>" + line);
					} else if (line.startsWith('Created')) {
						$("#jobBagLocation").show();
						$("#jobBagLocation .message").html(line);
						result.succeeded = true;
					}
				}
				// console.log(decoder.decode(data));
			});

			bagger.stderr.on('data', (data) => {
				$("#jobError").show()
				var lines = decoder.decode(data).split(NEWLINE);
				for (var line of lines) {
					$("#jobError").append(line + "<br/>")
				}
				result.error += lines;
			});

			// Send the job to the bagging program
			bagger.stdin.write(JSON.stringify(this));

		}
		if (this.storageServices.length > 0) {
			// Store it
		}
	}
}

function updateFileStats(stats, row) {
	updateStats(row, '.fileCount', 1)
	updateStats(row, '.fileSize', stats.size)
}

function updateStats(row, cssClass, amountToAdd) {
	var cell = $(row).find(cssClass).first()
	var prevValue = parseInt(cell.data('total'), 10) || 0
	var newValue = prevValue + amountToAdd
	cell.data('total', newValue)
	if (cssClass.indexOf('Count') > 0) {
		cell.text(newValue)
	} else {
		cell.text(formatFileSize(newValue))
	}

	var totalCell = getTotalCell(cssClass)
	prevValue = parseInt(totalCell.data('total'), 10) || 0
	newValue = prevValue + amountToAdd
	totalCell.data('total', newValue)
	if (cssClass.indexOf('Count') > 0) {
		totalCell.text(newValue)
	} else {
		totalCell.text(formatFileSize(newValue))
	}
}

function getTotalCell(cssClass) {
	switch(cssClass) {
	case '.dirCount':
		return $('#totalDirCount')
	case '.fileCount':
		return $('#totalFileCount')
	case '.fileSize':
		return $('#totalFileSize')
	}
	return null
}

function updateStatsForDeletion(row) {
	var dirCountCell = $(row).children('.dirCount').first()
	var fileCountCell = $(row).children('.fileCount').first()
	var sizeCell = $(row).children('.fileSize').first()
	var fileCount = parseInt(fileCountCell.data('total'), 10) || 0
	var size = parseInt(sizeCell.data('total'), 10) || 0
	var dirCount = parseInt(dirCountCell.data('total'), 10) || 0
	var totalDirCountCell = $('#totalDirCount')
	var prevTotalDirCount = parseInt(totalDirCountCell.data('total'), 10) || 0
	totalDirCountCell.data('total', (prevTotalDirCount - dirCount))
	totalDirCountCell.text(prevTotalDirCount - dirCount)
	var totalFileCountCell = $('#totalFileCount')
	var prevTotalFileCount = parseInt(totalFileCountCell.data('total'), 10) || 0
	totalFileCountCell.data('total', (prevTotalFileCount - fileCount))
	totalFileCountCell.text(prevTotalFileCount - fileCount)
	var totalSizeCell = $('#totalFileSize')
	var prevTotalSize = parseInt(totalSizeCell.data('total'), 10) || 0
	totalSizeCell.data('total', (prevTotalSize - size))
	totalSizeCell.text(formatFileSize(prevTotalSize - size))
}

function formatFileSize(size) {
	if (size > tb) {
		return (size / tb).toFixed(2) + " TB"
	}
	if (size > gb) {
		return (size / gb).toFixed(2) + " GB"
	}
	if (size > mb) {
		return (size / mb).toFixed(2) + " MB"
	}
	return (size / kb).toFixed(2) + " KB"
}

function getTableRow(filepath, isDir) {
	var icon = getIconForPath(filepath, isDir)
	return `<tr data-filepath="${filepath}" data-object-type="File">
		<td>${icon}</td>
		<td class="dirCount">0</td>
		<td class="fileCount">0</td>
		<td class="fileSize">0</td>
		<td class="deleteCell"><span class="glyphicon glyphicon-remove clickable-row" aria-hidden="true"></td>
		</tr>`
}

function getIconForPath(filepath, isDir) {
	if (isDir) {
		return getFolderIcon(filepath)
	}
	return getFileIcon(filepath)
}

function getFileIcon(filepath) {
	return '<span class="glyphicon glyphicon-file" aria-hidden="true" style="margin-right:10px"></span>' + filepath;
}

function getFolderIcon(filepath) {
	return '<span class="glyphicon glyphicon-folder-close" aria-hidden="true" style="margin-right:10px"></span>' + filepath;
}