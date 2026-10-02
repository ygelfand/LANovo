vec4 iris(float t) {
	float k = u[5], I = u[4];
	vec4 s0 = vec4(1.0, 250.0 / 255.0, 214.0 / 255.0, 1.0);
	vec4 s1 = vec4(1.0, 196.0 / 255.0, 72.0 / 255.0, 1.0);
	vec4 s2 = vec4(1.0, 72.0 / 255.0, 18.0 / 255.0, 1.0);
	float a3 = min(1.0, 0.72 + 0.28 * I), a4 = min(1.0, 0.5 + 0.4 * I);
	vec4 s3 = vec4(vec3(206.0, 8.0, 0.0) / 255.0 * a3, a3);
	vec4 s4 = vec4(vec3(96.0, 0.0, 0.0) / 255.0 * a4, a4);
	vec4 s5 = vec4(vec3(28.0, 0.0, 0.0) / 255.0 * 0.92, 0.92);
	vec4 s6 = vec4(5.0 / 255.0, 0.0, 0.0, 1.0);
	float p1 = 0.045 * k, p2 = 0.12 * k, p3 = 0.28 * k;
	if (t < p1) return mix(s0, s1, t / p1);
	if (t < p2) return mix(s1, s2, (t - p1) / (p2 - p1));
	if (t < p3) return mix(s2, s3, (t - p2) / (p3 - p2));
	if (t < 0.58) return mix(s3, s4, (t - p3) / (0.58 - p3));
	if (t < 0.86) return mix(s4, s5, (t - 0.58) / 0.28);
	return mix(s5, s6, (t - 0.86) / 0.14);
}

vec4 eye(highp float d) {
	highp float rg = u[2];
	if (d >= rg + 0.5) return vec4(0.0);
	float cov = clamp(float(rg - d) + 0.5, 0.0, 1.0);
	return iris(float(d / rg)) * u[6] * cov;
}

void main() {
	highp float s = res.x / u[9];
	highp vec2 p = pixel() / s;
	highp vec2 q = p - vec2(u[0], u[1]);
	highp float d = length(q);
#ifdef LIGHT
	gl_FragColor = vec4(eye(d).rgb, 1.0);
#else
	vec3 col = texture2D(t0, vtex).rgb;
	highp float rg = u[2], rc = u[3];
	if (d < rc * 1.2) {
		vec4 e = eye(d);
		col = e.rgb + col * (1.0 - e.a);
		float sp = clamp(float((d - rg * 0.95) / (rc * 1.2 - rg * 0.95)), 0.0, 1.0);
		col += vec3(1.0, 36.0 / 255.0, 10.0 / 255.0) * 0.14 * u[4] * (1.0 - sp);
		highp vec2 g = (p - vec2(u[10], u[11])) / vec2(u[12], u[13]);
		if (g.x >= 0.0 && g.y >= 0.0 && g.x < 1.0 && g.y < 1.0) {
			vec4 gl = texture2D(t1, g);
			col = gl.rgb + col * (1.0 - gl.a);
		}
		highp float R = rg * 0.93, w = 1.5 * u[8];
		if (u[7] > 0.0 && abs(d - R) < w + 1.0 + 3.0 * u[8]) {
			float a = atan(float(q.y), float(q.x));
			highp float dist;
			if (a >= 0.18 * 3.14159265 && a <= 0.82 * 3.14159265) dist = abs(d - R);
			else dist = min(length(q - R * vec2(0.84433, 0.53583)), length(q - R * vec2(-0.84433, 0.53583)));
			col += vec3(120.0, 215.0, 255.0) / 255.0 * u[7] * clamp(float(w - dist) + 0.5, 0.0, 1.0);
		}
	}
	gl_FragColor = vec4(min(col + glow(), 1.0), 1.0);
#endif
}
