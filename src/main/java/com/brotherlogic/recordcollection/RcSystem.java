package com.brotherlogic.recordcollection;

import com.brotherlogic.recordcollection.storage.Storage;

/**
 * Class to define the system
 */
public interface RcSystem {
  String getVersion();
  Config getConfig();
  Storage getStorage();
}
