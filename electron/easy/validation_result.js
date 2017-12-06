module.exports = class ValidationResult {
    constructor(errors) {
        this.errors = {};
    }
    objectType() {
        return 'ValidationResult';
    }
    isValid() {
        return Object.keys(this.errors).length == 0;
    }
}
