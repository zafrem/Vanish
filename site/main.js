// Reveal the copy buttons only when JavaScript and the Clipboard API exist.
(function () {
  if (!navigator.clipboard || !navigator.clipboard.writeText) return;

  document.querySelectorAll('button.copy[data-copy-target]').forEach(function (button) {
    var target = document.getElementById(button.getAttribute('data-copy-target'));
    if (!target) return;
    button.hidden = false;
    button.addEventListener('click', function () {
      navigator.clipboard.writeText(target.textContent.trim()).then(function () {
        button.textContent = 'Copied';
        setTimeout(function () { button.textContent = 'Copy'; }, 1500);
      }).catch(function () {
        button.textContent = 'Press Ctrl+C';
      });
    });
  });
})();
