document.addEventListener('click', (event) => {
	const shown = document.querySelector('.zoom-overlay');
	if (shown) {
		shown.remove();
		return;
	}
	const image = event.target.closest('.sl-markdown-content img');
	if (!image || image.closest('a')) return;
	const overlay = document.createElement('div');
	overlay.className = 'zoom-overlay';
	const big = document.createElement('img');
	big.src = image.currentSrc || image.src;
	big.alt = image.alt;
	overlay.append(big);
	document.body.append(overlay);
});

document.addEventListener('keydown', (event) => {
	if (event.key === 'Escape') document.querySelector('.zoom-overlay')?.remove();
});
