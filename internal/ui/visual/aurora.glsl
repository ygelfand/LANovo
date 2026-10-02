#ifdef PRE
precision highp float;
#endif

const highp float TAU = 6.2831853;

highp float hash(highp float i) {
	i = fract(mod(i, 1024.0) * 0.1031);
	i *= i + 33.33;
	i *= i + i;
	return fract(i);
}

highp float noise1(highp float x) {
	highp float i = floor(x);
	highp float f = x - i;
	f = f * f * (3.0 - 2.0 * f);
	return mix(hash(i), hash(i + 1.0), f);
}

#ifdef PRE
vec3 column(float xn, float y0, float a1, float k1, float a2, float k2, float hr, float own,
	float p1, float p2, float nHem, float nFold, float nRays, float nH) {
	float y = y0 + a1 * sin(k1 * xn * TAU + p1) + a2 * sin(k2 * xn * TAU + p2) + 0.044 * (noise1(xn * 11.0 + nHem) - 0.5);
	for (int i = 0; i < 16; i++) {
		float amp = u[42 + 4 * i];
		if (amp <= 0.0 || abs(u[43 + 4 * i] - own) > 0.5) continue;
		float age = u[41 + 4 * i];
		float d1 = xn - (u[40 + 4 * i] + 0.33 * age);
		float d2 = xn - (u[40 + 4 * i] - 0.33 * age);
		y -= amp * (exp(-d1 * d1 / 0.004) + exp(-d2 * d2 / 0.004)) * exp(-age * 1.1);
	}
	float e = own > 0.5 ? u[1] : u[0];
	float fold = noise1(xn * 5.0 + nFold);
	float rays = 0.55 + 0.45 * noise1(xn * 38.0 + nRays);
	float I = e * (0.25 + 0.75 * fold * fold) * rays * smoothstep(0.0, 0.07, xn) * (1.0 - smoothstep(0.93, 1.0, xn));
	float h = hr * (0.55 + 0.45 * noise1(xn * 3.0 + nH)) * (0.75 + 0.45 * min(e, 1.2));
	return vec3(y, h, I < 0.01 ? 0.0 : clamp(I, 0.0, 1.0));
}

void main() {
	float xn = gl_FragCoord.x / res.x;
	float k = floor(gl_FragCoord.y);
	vec3 c = k < 0.5 ? column(xn, 0.52, 0.05, 0.9, 0.03, 2.3, 0.4, 0.0, u[10], u[11], u[12], u[13], u[14], u[15])
		: k < 1.5 ? column(xn, 0.6, 0.04, 1.4, 0.025, 3.1, 0.3, 0.0, u[16], u[17], u[18], u[19], u[20], u[21])
		: k < 2.5 ? column(xn, 0.45, 0.06, 0.7, 0.035, 1.9, 0.38, 1.0, u[22], u[23], u[24], u[25], u[26], u[27])
		: column(xn, 0.56, 0.04, 1.7, 0.02, 2.7, 0.28, 1.0, u[28], u[29], u[30], u[31], u[32], u[33]);
	gl_FragColor = vec4(enc16((c.x + 0.5) / 2.0), c.y / 0.6, c.z);
}
#else

vec3 ribbon(vec2 q, vec4 a, float own) {
	if (a.a <= 0.0) return vec3(0.0);
	float y = a.r * 0.99609 + a.g * 0.00389 - 0.25;
	float t = (2.0 * y - q.y) / (a.b * 0.6);
	if (t < 0.0 || t > 1.0) return vec3(0.0);
	vec4 c = texture2D(t2, vec2((t * 127.0 + 0.5) / 128.0, (own + 0.5) / 2.0));
	return c.rgb * c.a * a.a;
}

vec3 aurora(vec2 q) {
	return ribbon(q, texture2D(pre, vpre0), 0.0) + ribbon(q, texture2D(pre, vpre1), 0.0)
		+ ribbon(q, texture2D(pre, vpre2), 1.0) + ribbon(q, texture2D(pre, vpre3), 1.0);
}

void main() {
	vec2 q = vtex;
	vec3 aur = aurora(q);
#ifdef LIGHT
	gl_FragColor = vec4(aur, 1.0);
#else
	vec4 base = texture2D(t0, vtex);
	vec3 col = base.rgb;
	bool upper = q.y < 0.76;
	if (upper && base.a > 0.0) {
		vec4 s = texture2D(t1, vtex);
		highp float ph = u[2] * s.g * 3.0 + s.b * TAU;
		float b = s.r * (0.55 + 0.45 * sin(ph));
		col = mix(col, vec3(220.0, 235.0, 255.0) / 255.0, b * base.a);
	}
	if (u[7] > 0.0) {
		highp vec2 p = pixel();
		highp vec2 a = vec2(u[3], u[4]) * res, b = vec2(u[5], u[6]) * res;
		highp vec2 d = b - a;
		highp float t = clamp(dot(p - a, d) / dot(d, d), 0.0, 1.0);
		float cov = clamp(0.8 * min(res.x, res.y) / 800.0 + 0.5 - length(p - a - d * t), 0.0, 1.0);
		col = mix(col, vec3(1.0), u[7] * float(t) * cov);
	}
	col = min(col + aur + glow(), 1.0);
	float k = q.y >= 0.78 && q.y < 0.93 ? (q.y - 0.78) / 0.15 : 0.0;
	vec3 hc = u[9] > 0.5 ? vec3(200.0, 90.0, 220.0) / 255.0 : vec3(80.0, 220.0, 160.0) / 255.0;
	col = min(col + hc * u[8] * k, 1.0);
	if (!upper && base.a > 0.0) col = mix(col, vec3(1.0, 3.0, 10.0) / 255.0, base.a);
	gl_FragColor = vec4(col, 1.0);
#endif
}
#endif
