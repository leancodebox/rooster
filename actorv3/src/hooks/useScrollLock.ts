import { useEffect, useRef } from 'react';

export function useScrollLock(isLocked: boolean) {
  // 记录原始 overflow 值，以便恢复
  const originalOverflow = useRef('');

  useEffect(() => {
    if (isLocked) {
      // 只有在第一次锁定时才保存原始值
      if (document.body.style.overflow !== 'hidden') {
        originalOverflow.current = document.body.style.overflow;
        document.body.style.overflow = 'hidden';
      }
    } else {
      // 只有在确实被锁定时才恢复
      if (document.body.style.overflow === 'hidden') {
        document.body.style.overflow = originalOverflow.current;
      }
    }

    // 组件卸载或状态变化时清理
    return () => {
      // 如果当前是锁定状态，组件卸载时应恢复
      if (isLocked && document.body.style.overflow === 'hidden') {
        document.body.style.overflow = originalOverflow.current;
      }
    };
  }, [isLocked]);
}
