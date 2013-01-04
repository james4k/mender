(function(exports) {
	exports.Dependency = {
		fn: function() {}
	};
})(window);

(function(exports) {
	exports.SomeApp = {
		something: function() {
			Dependency.fn();
		}
	};
})(window);

