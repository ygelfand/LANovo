vec3 ramp(float p) {
	if (p <= 0.45) return mix(vec3(34.0, 190.0, 40.0), vec3(120.0, 222.0, 30.0), p / 0.45) / 255.0;
	if (p <= 0.62) return mix(vec3(120.0, 222.0, 30.0), vec3(250.0, 226.0, 30.0), (p - 0.45) / 0.17) / 255.0;
	if (p <= 0.8) return mix(vec3(250.0, 226.0, 30.0), vec3(255.0, 150.0, 22.0), (p - 0.62) / 0.18) / 255.0;
	if (p <= 0.9) return mix(vec3(255.0, 150.0, 22.0), vec3(255.0, 72.0, 20.0), (p - 0.8) / 0.1) / 255.0;
	return mix(vec3(255.0, 72.0, 20.0), vec3(255.0, 30.0, 20.0), (p - 0.9) / 0.1) / 255.0;
}

vec3 led(highp vec2 p) {
	highp float x0 = u[64], colW = u[65], base = u[66], segH = u[67];
	highp float cx = (p.x - x0) / colW;
	if (cx < 0.0 || cx >= 32.0) return vec3(0.0);
	highp float ky = (base - p.y) / segH;
	if (ky < 0.0 || ky >= 22.0) return vec3(0.0);
	float i = floor(float(cx)), k = floor(float(ky));
	highp float gap = colW * 0.2, sg = max(1.0, segH * 0.26);
	highp float lx = p.x - x0 - i * colW, ly = (base - p.y) - k * segH;
	float cov = clamp(float(min(lx - gap * 0.5, gap * 0.5 + colW - gap - lx)) + 0.5, 0.0, 1.0)
		* clamp(float(min(ly - sg * 0.5, segH - sg * 0.5 - ly)) + 0.5, 0.0, 1.0);
	if (cov <= 0.0) return vec3(0.0);
	int ci = int(i);
	float h = u[ci], pk = u[32 + ci];
	float n = floor(h * 22.0 + 0.5);
	float pki = clamp(ceil(pk * 22.0) - 1.0, 0.0, 21.0);
	vec3 c = ramp(k / 21.0);
	vec3 col = k < n ? c : (k == pki && pk > 0.05) ? vec3(1.0, 38.0 / 255.0, 24.0 / 255.0) : c * 0.15;
	return col * cov;
}

vec3 scene(highp vec2 p) {
	highp float base = u[66], top = u[68], gapY = u[69];
	highp float floorY = base + gapY * 0.5;
	vec3 col = vec3(0.0);
	if (p.y > floorY) {
		col = led(vec2(p.x, 2.0 * base + gapY - p.y)) * 0.5;
		float f = clamp(float((p.y - floorY) / ((base - top) * 0.55)), 0.0, 1.0);
		col *= 1.0 - mix(0.3, 1.0, f);
		highp float x0 = u[64], span = u[65] * 32.0;
		if (abs(p.y - floorY) < max(1.0, u[72]) * 0.5 && p.x > x0 && p.x < x0 + span) {
			float e = 1.0 - abs(float((p.x - x0) / span) * 2.0 - 1.0);
			col += vec3(160.0, 255.0, 170.0) / 255.0 * 0.16 * e;
		}
	} else {
		col = led(p);
	}
	return min(col, 1.0);
}

void main() {
	highp vec2 p = pixel() * (u[70] / res.x);
#ifdef LIGHT
	gl_FragColor = vec4(scene(p), 1.0);
#else
	gl_FragColor = vec4(min(scene(p) + glow(), 1.0), 1.0);
#endif
}
