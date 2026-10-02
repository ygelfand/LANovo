highp float hash(highp vec2 q) {
	highp vec3 p3 = fract(vec3(q.xyx) * 0.1031);
	p3 += dot(p3, p3.yzx + 33.33);
	return fract((p3.x + p3.y) * p3.z);
}

#define GHOST(k, tk, c) { vec2 g = abs(d - ax * tk) / u[14 + k]; if (g.x < 1.2 && g.y < 1.2) { float h = max(g.x * 0.866 + g.y * 0.5, g.y); col += c * (smoothstep(1.0, 0.9, h) * (0.3 + 0.7 * smoothstep(0.2, 1.0, h))) * I * 0.16; } }

vec3 hot(vec2 d, float r) {
	vec3 tint = vec3(u[10], u[11], u[12]);
	float I = u[6];
	vec3 col = vec3(0.0);
	if (r < 0.5) {
		float k = 0.35 * exp(-r * 14.0);
		if (r < 0.06) k += 1.4 * exp(-r * r * 2200.0);
		col = mix(vec3(1.0), tint, 0.35) * I * k;
	}
	float sy = abs(d.y) / 0.0035;
	if (sy < 3.5) col += mix(tint, vec3(0.45, 0.65, 1.0), 0.6) * I * 0.9 * exp(-sy * sy - abs(d.x) / u[7] * 2.2);
	return col;
}

float ray(float c) {
	c *= c;
	c *= c;
	c *= c;
	c *= c;
	c *= c;
	return c * c;
}

vec3 scene(highp vec2 p, vec2 d, float r) {
	vec3 tint = vec3(u[10], u[11], u[12]);
	float I = u[6];
	vec2 ax = vec2(u[23], u[24]);

	vec3 col = mix(vec3(0.012, 0.02, 0.05), vec3(0.03, 0.045, 0.09), float(vtex.y));
	highp vec2 cell = floor(p / u[25]);
	float hsh = float(hash(cell));
	if (hsh > 0.994) {
		float tw = 0.5 + 0.5 * sin(u[13] * (1.0 + hsh * 3.0) + hsh * 40.0);
		col += vec3(0.8, 0.85, 1.0) * (0.35 + 0.65 * tw) * max(0.0, 1.0 - float(length(p - (cell + 0.5) * u[25]) / (u[25] * 0.3)));
	}

	col += hot(d, r);
	if (r < 0.95) {
		vec2 n = d / max(r, 1e-3);
		float c2 = n.x * n.x - n.y * n.y, s2 = 2.0 * n.x * n.y;
		float c3 = c2 * n.x - s2 * n.y, s3 = s2 * n.x + c2 * n.y;
		float c5 = c3 * c2 - s3 * s2, s5 = s3 * c2 + c3 * s2;
		float r1 = ray(abs(c3 * u[8] - s3 * u[9]));
		float r2 = ray(abs(c5 * u[21] + s5 * u[22]));
		col += tint * (r1 + 0.6 * r2 * r2) * exp(-r * 5.0) * I * 0.55;
	}

	GHOST(0, 0.45, vec3(0.55, 0.95, 0.5))
	GHOST(1, 0.7, vec3(0.95, 0.5, 0.85))
	GHOST(2, 0.95, vec3(0.5, 0.7, 1.0))
	GHOST(3, 1.25, vec3(1.0, 0.7, 0.4))
	GHOST(4, 1.55, vec3(0.55, 1.0, 0.9))
	GHOST(5, 1.9, vec3(0.8, 0.6, 1.0))

	float e = length(d - ax) - 0.465;
	if (abs(e) < 0.04) {
		vec3 q = (vec3(e) + vec3(0.01, 0.0, -0.01)) / 0.01;
		col += exp(-q * q) * u[20] * I * 0.1;
	}

	vec2 vq = vec2(vtex) * 2.0 - 1.0;
	col *= 1.0 - 0.35 * dot(vq, vq);
	return col;
}

void main() {
	highp vec2 p = pixel() * (u[0] / res.x);
	vec2 d = vec2((p - vec2(u[2], u[3])) / u[1]);
	float r = length(d);
#ifdef LIGHT
	gl_FragColor = vec4(hot(d, r), 1.0);
#else
	gl_FragColor = vec4(min(scene(p, d, r) + glow(), 1.0), 1.0);
#endif
}
