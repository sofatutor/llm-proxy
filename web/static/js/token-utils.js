function copyToClipboard(text) {
    if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
        navigator.clipboard.writeText(text).then(function() {
            showCopyToast();
        }).catch(function(err) {
            console.error('Failed to copy: ', err);
        });
    } else {
        try {
            var textarea = document.createElement('textarea');
            textarea.value = text;
            textarea.style.position = 'fixed';
            document.body.appendChild(textarea);
            textarea.focus();
            textarea.select();
            var successful = document.execCommand('copy');
            document.body.removeChild(textarea);
            if (successful) {
                showCopyToast();
            } else {
                console.error('Fallback: Copy command was unsuccessful');
            }
        } catch (err) {
            console.error('Fallback: Oops, unable to copy', err);
        }
    }
}

function showCopyToast() {
    var toast = document.createElement('div');
    toast.className = 'toast position-fixed top-0 end-0 m-3';
    toast.setAttribute('role', 'alert');
    toast.innerHTML = '<div class="toast-body bg-success text-white">' +
                      '<i class="bi bi-check-circle"></i> Copied to clipboard!' +
                      '</div>';
    document.body.appendChild(toast);
    if (typeof bootstrap !== 'undefined' && bootstrap.Toast) {
        var bsToast = new bootstrap.Toast(toast);
        bsToast.show();
        toast.addEventListener('hidden.bs.toast', function() { toast.remove(); });
    }
}

function revokeToken(tokenId, obfuscatedToken) {
    if (confirm('Are you sure you want to revoke token ' + obfuscatedToken + '? This action cannot be undone.')) {
        var form = document.createElement('form');
        form.method = 'POST';
        form.action = '/tokens/' + tokenId;
        var methodInput = document.createElement('input');
        methodInput.type = 'hidden';
        methodInput.name = '_method';
        methodInput.value = 'DELETE';
        form.appendChild(methodInput);
        document.body.appendChild(form);
        form.submit();
    }
}

function bulkRevokeTokens(projectId, projectName) {
    if (confirm('Are you sure you want to revoke ALL tokens for project "' + projectName + '"? This action cannot be undone.')) {
        var form = document.createElement('form');
        form.method = 'POST';
        form.action = '/projects/' + projectId + '/revoke-tokens';
        document.body.appendChild(form);
        form.submit();
    }
}
