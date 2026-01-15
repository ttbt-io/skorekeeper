// Copyright (c) 2026 TTBT Enterprises LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

(function() {
    window.sk_errors = [];
    const MAX_ERRORS = 50;

    // Exposed API
    window.SkErrorLogger = {
        getErrors: () => window.sk_errors,
        onError: null, // Callback for UI
        dismissAll: false,
    };

    function logError(type, message, source, lineno, colno, error) {
        const errObj = {
            timestamp: new Date().toISOString(),
            type: type,
            message: message,
            source: source,
            lineno: lineno,
            colno: colno,
            stack: error ? error.stack : null,
        };
        window.sk_errors.push(errObj);
        if (window.sk_errors.length > MAX_ERRORS) {
            window.sk_errors.shift();
        }

        if (window.SkErrorLogger.onError && !window.SkErrorLogger.dismissAll) {
            window.SkErrorLogger.onError(errObj);
        }
    }

    window.onerror = function(message, source, lineno, colno, error) {
        logError('uncaught', message, source, lineno, colno, error);
        // Returning false allows the browser's default error handler to run (logging to console).
        return false;
    };

    window.onunhandledrejection = function(event) {
        logError('unhandledrejection', event.reason?.message || event.reason || 'Unknown reason', null, null, null, event.reason);
        // Not calling event.preventDefault() ensures it still shows up in the console.
    };
})();
