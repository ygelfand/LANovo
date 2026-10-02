const float N = 54.0;

vec3 scene(highp vec2 p, highp vec2 uv) {
	vec3 col = texture2D(t0, uv).rgb;
	highp float top = u[0], gap = u[1], A = u[2], fr = u[3], x0 = u[4], pw = u[5];
	float w = u[6];
	highp float xn = (p.x - x0) / pw;
	if (xn < 0.0 || xn > 1.0) return col;
	highp float base = (p.y - top) / gap - 1.0 + fr;
	highp float jmax = min(N, floor(base + 1.4 * A / gap) + 1.0);
	highp float jmin = max(0.0, ceil(base - w / gap));
	highp float tx = (xn * 79.0 + 0.5) / 80.0;
	vec3 tint = vec3(u[8], u[9], u[10]);
	for (int k = 0; k < 16; k++) {
		highp float j = jmax - float(k);
		if (j < jmin) break;
		highp vec4 c = texture2D(t1, vec2(tx, (j + 0.5) / 56.0));
		highp float h = (c.r * 65280.0 + c.g * 255.0) / 32767.5;
		highp float yj = top + (j + 1.0 - fr) * gap;
		highp float d = p.y - (yj - h * A);
		if (d > -w * 0.5 - 0.5 && p.y <= yj + w) {
			float cov = clamp(w * 0.5 - float(abs(d)) + 0.5, 0.0, 1.0);
			float near = float(j / N);
			float alpha = j < 0.5 ? 1.0 - fr : j > N - 0.5 ? min(1.0, fr * 3.0 + 0.25) : 1.0;
			vec3 lc = mix(vec3(236.0) / 255.0, tint, pow(near, 5.0)) * alpha * (0.8 + 0.2 * near);
			return lc * cov + (p.y < yj ? vec3(0.0) : col) * (1.0 - cov);
		}
	}
	return col;
}

void main() {
	highp float s = res.x / u[7];
	highp vec2 p = pixel() / s;
#ifdef LIGHT
	gl_FragColor = vec4(scene(p, vtex), 1.0);
#else
	gl_FragColor = vec4(min(scene(p, vtex) + glow(), 1.0), 1.0);
#endif
}
