function enlarge(image) {
	const overlay = document.createElement('dialog');
	overlay.className = 'zoom-overlay';
	overlay.setAttribute('aria-label', image.alt || 'Screenshot');
	const big = document.createElement('img');
	big.src = image.currentSrc || image.src;
	big.alt = image.alt;
	const size = document.createElement('button');
	size.textContent = 'Actual size';
	size.className = 'zoom-size';
	size.addEventListener('click', () => {
		size.textContent = overlay.classList.toggle('zoom-natural') ? 'Fit to screen' : 'Actual size';
		overlay.scrollTo(0, 0);
	});
	const close = document.createElement('button');
	close.textContent = 'Close';
	close.className = 'zoom-close';
	close.addEventListener('click', () => overlay.close());
	overlay.addEventListener('click', (event) => {
		if (event.target === overlay) overlay.close();
	});
	overlay.addEventListener('close', () => {
		overlay.remove();
		image.focus({ preventScroll: true });
	});
	overlay.append(big, size, close);
	document.body.append(overlay);
	overlay.showModal();
}

function prepareImages() {
	for (const image of document.querySelectorAll('.sl-markdown-content img')) {
		if (image.closest('a')) continue;
		image.tabIndex = 0;
		image.setAttribute('role', 'button');
		image.setAttribute('aria-label', `Enlarge: ${image.alt || 'screenshot'}`);
	}
}

document.addEventListener('click', (event) => {
	const image = event.target.closest('.sl-markdown-content img[role="button"]');
	if (image) enlarge(image);
});

document.addEventListener('keydown', (event) => {
	if ((event.key === 'Enter' || event.key === ' ') && event.target.matches('.sl-markdown-content img[role="button"]')) {
		event.preventDefault();
		enlarge(event.target);
	}
});

prepareImages();
document.addEventListener('astro:page-load', prepareImages);
