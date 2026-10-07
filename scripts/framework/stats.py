"""Two-sided 95% Student-t interval half-width, for the small repeat counts of the live runs."""

# Critical values by degrees of freedom; beyond 30 the normal value is approached slowly, so use coarse steps.
_T975 = [12.706, 4.303, 3.182, 2.776, 2.571, 2.447, 2.365, 2.306, 2.262, 2.228,
         2.201, 2.179, 2.160, 2.145, 2.131, 2.120, 2.110, 2.101, 2.093, 2.086,
         2.080, 2.074, 2.069, 2.064, 2.060, 2.056, 2.052, 2.048, 2.045, 2.042]


def t975(df: int) -> float:
    if df < 1:
        raise ValueError("need at least two samples")
    if df <= 30:
        return _T975[df - 1]
    if df <= 60:
        return 2.000
    if df <= 120:
        return 1.980
    return 1.960


def ci95_halfwidth(stdev: float, n: int) -> float:
    return 0.0 if n < 2 else t975(n - 1) * stdev / n ** 0.5
